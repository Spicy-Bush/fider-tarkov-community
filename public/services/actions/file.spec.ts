import { deleteFiles, pruneFiles, uploadFile } from "./file"
import { http, RequestError } from "@fider/services/http"
import { setImmediate } from "node:timers"

jest.mock("@fider/services/http", () => ({
  ...jest.requireActual("@fider/services/http"),
  http: { post: jest.fn() },
}))

const post = jest.mocked(http.post)
beforeEach(() => {
  post.mockReset()
  jest.useFakeTimers()
})

afterEach(() => jest.useRealTimers())

test("an uncertain upload is retried automatically with the same identity and bytes", async () => {
  post.mockRejectedValueOnce(new RequestError("POST", "/api/admin/files", "transport", new Error("response lost")))
  post.mockResolvedValue({ ok: true, data: { blobKey: "files/confirmed" } })
  const request = { submissionId: "one-operation", name: "Screenshot", uploadType: "file" as const, file: { upload: { content: "image" }, remove: false } }

  const completion = uploadFile(request)
  await new Promise<void>(resolve => setImmediate(resolve))
  jest.advanceTimersByTime(250)

  expect((await completion).ok).toBe(true)
  expect(post).toHaveBeenCalledTimes(2)
  expect(post.mock.calls[1]).toEqual(post.mock.calls[0])
})

test("deletion sends the selected keys and cleanup scope", async () => {
  post.mockResolvedValue({ ok: true, data: { deleted: ["files/selected"] } })
  const removal = { force: false, includeDeleted: true, includeDrafts: false }

  expect((await deleteFiles(["files/selected"], removal)).ok).toBe(true)
  expect(post).toHaveBeenCalledTimes(1)
  expect(post).toHaveBeenCalledWith("/api/admin/files/delete", {
    blobKeys: ["files/selected"],
    ...removal,
  }, { notifyOnError: false })
})

test("an unconfirmed cleanup batch retries its original scope and cursor", async () => {
  post.mockRejectedValueOnce(new RequestError("POST", "/api/admin/files/prune", "transport", new Error("response lost")))
  post.mockResolvedValue({ ok: true, data: { deleted: ["files/confirmed"], pending: [], skipped: [], errors: [] } })
  const request = {
    search: "Screenshot", type: "files" as const, before: "2026-09-28T00:00:00Z", cursor: "last-reviewed-key",
    includeDeleted: true, includeDrafts: false,
  }

  const completion = pruneFiles(request)
  await new Promise<void>(resolve => setImmediate(resolve))
  jest.advanceTimersByTime(250)

  expect((await completion).ok).toBe(true)
  expect(post).toHaveBeenCalledTimes(2)
  expect(post.mock.calls[1]).toEqual(post.mock.calls[0])
})
