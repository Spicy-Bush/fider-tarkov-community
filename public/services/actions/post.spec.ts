import { beforeEach, expect, test } from "@jest/globals"
import { searchPosts, setVote } from "./post"
import { http } from "@fider/services/http"
import { RequestError } from "@fider/services/http"

jest.mock("@fider/services/http", () => ({
  ...jest.requireActual("@fider/services/http"),
  http: { get: jest.fn(), post: jest.fn(), delete: jest.fn() },
}))

beforeEach(() => jest.resetAllMocks())

test("record reads retain search criteria and forward cancellation", async () => {
  const controller = new AbortController()
  jest.mocked(http.get).mockResolvedValueOnce({ ok: true, data: [] })

  await searchPosts({ ids: [7, 5], statuses: ["open"], query: "example", tags: ["planned"] }, {
    signal: controller.signal,
    notifyOnError: false,
  })

  const [url, options] = jest.mocked(http.get).mock.calls[0]
  const parameters = new URL(url, "https://example.test").searchParams
  expect(Object.fromEntries(parameters)).toEqual({
    ids: "7,5",
    statuses: "open",
    query: "example",
    tags: "planned",
  })
  expect(options).toEqual({ signal: controller.signal, includeHeaders: undefined, notifyOnError: false })
})

test("post searches forward cancellation and requested response headers", async () => {
  const controller = new AbortController()
  jest.mocked(http.get).mockResolvedValueOnce({ ok: true, data: [] })

  await searchPosts({ view: "newest", includeCount: true }, { signal: controller.signal })

  expect(http.get).toHaveBeenCalledWith(expect.stringContaining("includeCount=true"), {
    signal: controller.signal,
    includeHeaders: true,
  })
})

test("a lost acknowledgement retries the original revision without overwriting a newer vote", async () => {
  jest.mocked(http.post).mockRejectedValueOnce(new RequestError("POST", "/api/posts/1/down", "transport", new Error("connection lost")))
  const current = { direction: 1, revision: 9, upvotes: 13, downvotes: 2, applied: false }
  jest.mocked(http.post).mockResolvedValueOnce({ ok: true, data: current })

  await expect(setVote(1, -1, 7)).resolves.toEqual({ ok: true, data: current, unconfirmed: false })
  expect(http.post).toHaveBeenNthCalledWith(1, "/api/posts/1/down", { revision: 7 })
  expect(http.post).toHaveBeenNthCalledWith(2, "/api/posts/1/down", { revision: 7 })
})

test("a rejected vote and a programming error do not trigger another write", async () => {
  const rejected = { ok: false as const, status: 403, error: { errors: [{ message: "Post locked" }] } }
  jest.mocked(http.post).mockResolvedValueOnce(rejected)
  await expect(setVote(1, 1, 2)).resolves.toMatchObject({ ...rejected, unconfirmed: false })
  expect(http.post).toHaveBeenCalledTimes(1)

  const defect = new Error("presentation defect")
  jest.mocked(http.post).mockRejectedValueOnce(defect)
  await expect(setVote(1, 1, 2)).rejects.toBe(defect)
  expect(http.post).toHaveBeenCalledTimes(2)
})

test("removing a vote sends its revision through the existing delete route", async () => {
  const response = { ok: true as const, data: { direction: 0, revision: 4, upvotes: 2, downvotes: 0, applied: true } }
  jest.mocked(http.delete).mockResolvedValueOnce(response)
  await expect(setVote(1, 0, 3)).resolves.toEqual({ ...response, unconfirmed: false })
  expect(http.delete).toHaveBeenCalledWith("/api/posts/1/votes", { revision: 3 })
})


test("a denied retry preserves the lost acknowledgement and original revision", async () => {
  const cause = new RequestError("POST", "/api/posts/1/up", "transport", new Error("lost acknowledgement"))
  jest.mocked(http.post).mockRejectedValueOnce(cause)
  jest.mocked(http.post).mockResolvedValueOnce({ ok: false, status: 403, error: { errors: [{ message: "Post locked" }] } })

  await expect(setVote(1, 1, 7)).resolves.toMatchObject({
    ok: false, status: 403, unconfirmed: true, error: { errors: [{ message: "Post locked" }], cause },
  })
  expect(jest.mocked(http.post).mock.calls).toEqual([
    ["/api/posts/1/up", { revision: 7 }],
    ["/api/posts/1/up", { revision: 7 }],
  ])
})
