import { test, expect, beforeEach } from "@jest/globals"
import { http, RequestError } from "./http"
import * as notify from "./notify"
import { allocateSponsors } from "./actions/sponsorBooking"

jest.mock("./analytics", () => ({ analytics: { event: jest.fn() } }))
jest.mock("./notify", () => ({ error: jest.fn() }))

const fetchMock = jest.fn<typeof fetch>()
beforeEach(() => {
  jest.clearAllMocks()
  global.fetch = fetchMock
})

function response(status: number, body: unknown, failure?: Error): Response {
  return {
    status,
    ok: status >= 200 && status < 300,
    headers: { get: () => null } as unknown as Headers,
    json: async () => {
      if (failure) throw failure
      return body
    },
  } as Response
}

test("transport diagnostics preserve the cause without body or query secrets", async () => {
  const cause = new TypeError("connection lost")
  fetchMock.mockRejectedValueOnce(cause)
  let failure: RequestError | undefined
  try {
    await http.post("/api/posts?token=query-secret", { description: "private-draft" })
  } catch (error) {
    failure = error as RequestError
  }
  expect(failure).toBeInstanceOf(RequestError)
  expect(failure?.cause).toBe(cause)
  expect(failure?.phase).toBe("transport")
  expect(`${failure?.stack} ${JSON.stringify(failure)}`).not.toMatch(/query-secret|private-draft/)
})

test("an HTML gateway failure keeps its status and parsing cause", async () => {
  const cause = new SyntaxError("not JSON")
  fetchMock.mockResolvedValueOnce(response(502, undefined, cause))
  const result = await http.post("/api/posts", { description: "private" })
  expect(result.ok).toBe(false)
  if (result.ok) throw new Error("expected failure")
  expect(result.status).toBe(502)
  expect(result.error.cause).toBe(cause)
  expect(result.error.errors?.[0].message).toContain("502")
  expect(notify.error).toHaveBeenCalledTimes(1)
})

test.each([401, 403, 503])("callers can reconcile HTTP %i before deciding to display an error", async (status) => {
  fetchMock.mockResolvedValueOnce(response(status, { errors: [{ message: "Request rejected" }] }))

  const result = await http.post("/api/pages/1/comments", {}, { notifyOnError: false })

  expect(result).toMatchObject({ ok: false, status, error: { errors: [{ message: "Request rejected" }] } })
  expect(notify.error).not.toHaveBeenCalled()
})

test("sponsor allocation leaves failure presentation to its owner", async () => {
  fetchMock.mockResolvedValueOnce(response(503, { errors: [{ message: "Unavailable" }] }))

  const result = await allocateSponsors(
    { pageType: "home", id: 0, language: "en", device: "desktop" },
    [{ instanceId: "feed-0", placementId: "feed_desktop" }],
    "issued-page-token",
  )

  expect(result).toMatchObject({ ok: false, status: 503 })
  expect(fetchMock).toHaveBeenCalledWith("/api/sponsorship/select", expect.objectContaining({
    method: "POST",
  }))
  expect(notify.error).not.toHaveBeenCalled()
})

test("malformed successful JSON is a response error, while 204 needs no JSON", async () => {
  const cause = new SyntaxError("truncated JSON")
  fetchMock.mockResolvedValueOnce(response(200, undefined, cause))
  await expect(http.get("/api/posts")).rejects.toMatchObject({ phase: "response", status: 200, cause })
  fetchMock.mockResolvedValueOnce(response(204, undefined, cause))
  await expect(http.delete("/api/posts/1")).resolves.toMatchObject({ ok: true, data: undefined })
})

test("serialization and notification defects retain their original cause", async () => {
  const serialization = new Error("serializer defect")
  await expect(http.post("/api/posts", { toJSON() { throw serialization } })).rejects.toBe(serialization)
  expect(fetchMock).not.toHaveBeenCalled()
  const presentation = new Error("notification defect")
  jest.mocked(notify.error).mockImplementationOnce(() => { throw presentation })
  fetchMock.mockResolvedValueOnce(response(403, {}))
  await expect(http.get("/api/posts")).rejects.toBe(presentation)
})
