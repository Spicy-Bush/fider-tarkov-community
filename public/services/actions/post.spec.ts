import { beforeEach, expect, test } from "@jest/globals"
import { setVote } from "./post"
import { http } from "@fider/services"
import { RequestError } from "@fider/services/http"

jest.mock("@fider/services", () => ({
  http: { post: jest.fn(), delete: jest.fn() },
}))

beforeEach(() => jest.resetAllMocks())

test("a lost acknowledgement retries the original revision without overwriting a newer vote", async () => {
  jest.mocked(http.post).mockRejectedValueOnce(new RequestError("POST", "/api/v1/posts/1/down", "transport", new Error("connection lost")))
  const current = { direction: 1, revision: 9, upvotes: 13, downvotes: 2, applied: false }
  jest.mocked(http.post).mockResolvedValueOnce({ ok: true, data: current })

  await expect(setVote(1, -1, 7)).resolves.toEqual({ ok: true, data: current })
  expect(http.post).toHaveBeenNthCalledWith(1, "/api/v1/posts/1/down", { revision: 7 })
  expect(http.post).toHaveBeenNthCalledWith(2, "/api/v1/posts/1/down", { revision: 7 })
})

test("a rejected vote and a programming error do not trigger another write", async () => {
  const rejected = { ok: false as const, status: 403, error: { errors: [{ message: "Post locked" }] } }
  jest.mocked(http.post).mockResolvedValueOnce(rejected)
  await expect(setVote(1, 1, 2)).resolves.toEqual(rejected)
  expect(http.post).toHaveBeenCalledTimes(1)

  const defect = new Error("presentation defect")
  jest.mocked(http.post).mockRejectedValueOnce(defect)
  await expect(setVote(1, 1, 2)).rejects.toBe(defect)
  expect(http.post).toHaveBeenCalledTimes(2)
})

test("removing a vote sends its revision through the existing delete route", async () => {
  const response = { ok: true as const, data: { direction: 0, revision: 4, upvotes: 2, downvotes: 0, applied: true } }
  jest.mocked(http.delete).mockResolvedValueOnce(response)
  await expect(setVote(1, 0, 3)).resolves.toEqual(response)
  expect(http.delete).toHaveBeenCalledWith("/api/v1/posts/1/votes", { revision: 3 })
})
