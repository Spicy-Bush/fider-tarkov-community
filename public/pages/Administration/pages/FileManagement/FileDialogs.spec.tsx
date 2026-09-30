import React from "react"
import { webcrypto } from "crypto"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { DeleteFilesDialog, PruneFilesDialog, UploadFileDialog } from "./FileDialogs"
import { deleteFiles, FileInfo, pruneFiles, uploadFile, newFileUploadID } from "@fider/services/actions/file"
import { RequestError } from "@fider/services/http"

jest.mock("@fider/services/actions/file", () => ({
  ...jest.requireActual("@fider/services/actions/file"),
  deleteFiles: jest.fn(),
  pruneFiles: jest.fn(),
  uploadFile: jest.fn(),
  newFileUploadID: jest.fn(),
}))

const file = (name: string): FileInfo => ({
  name, blobKey: `files/${name}`, size: 20, contentType: "image/png", createdAt: "2026-09-28T00:00:00Z",
  width: 2, height: 2, thumbnailURL: `/thumb/${name}`, url: `/image/${name}`, isInUse: false, hasProtectedReferences: false, state: "ready",
})

const protectedScope = { includeDeleted: false, includeDrafts: false }

beforeEach(() => {
  jest.clearAllMocks()
  let uploads = 0
  jest.mocked(newFileUploadID).mockImplementation(async () => ({ ok: true, data: `server-upload-${++uploads}` }))
  Object.defineProperty(globalThis, "crypto", { configurable: true, value: webcrypto })
  const root = document.createElement("div")
  root.id = "root-modal"
  document.body.appendChild(root)
})

afterEach(() => {
  document.getElementById("root-modal")?.remove()
  jest.restoreAllMocks()
})

test("a failed image read remains editable and can recover before any upload is sent", async () => {
  jest.spyOn(FileReader.prototype, "readAsDataURL").mockImplementationOnce(function () {
    this.dispatchEvent(new ProgressEvent("error"))
  })
  const upload = jest.mocked(uploadFile)
    .mockResolvedValue({ ok: true, unconfirmed: false, data: file("Recovered image") })
  const onUploaded = jest.fn()
  render(<UploadFileDialog isOpen onClose={jest.fn()} onUploaded={onUploaded} />)

  fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Recovered image" } })
  fireEvent.change(document.querySelector('input[type="file"]')!, {
    target: { files: [new File(["image bytes"], "screenshot.png", { type: "image/png" })] },
  })
  await screen.findByText("Could not read screenshot.png.")
  fireEvent.click(screen.getByRole("button", { name: "Upload image", exact: true }))
  await screen.findByText("Select an image before uploading.")

  expect(upload).not.toHaveBeenCalled()
  expect(screen.getByLabelText("Name")).toBeEnabled()
  expect(screen.queryByRole("button", { name: "Retry upload" })).not.toBeInTheDocument()

  fireEvent.click(screen.getByRole("button", { name: "Retry image" }))
  await waitFor(() => expect(document.querySelector('img[src^="data:image/png"]')).not.toBeNull())
  fireEvent.click(screen.getByRole("button", { name: "Upload image", exact: true }))
  await waitFor(() => expect(onUploaded).toHaveBeenCalledTimes(1))
  expect(upload).toHaveBeenCalledTimes(1)
  expect(upload.mock.calls[0][0].file.upload?.content).toBe(btoa("image bytes"))
})

test("failed identity issuance preserves an editable image and recovers without sending an upload", async () => {
  jest.mocked(newFileUploadID).mockRejectedValueOnce(
    new RequestError("POST", "/api/uploads/id", "transport", new Error("offline"))
  )
  jest.mocked(uploadFile).mockResolvedValue({ ok: true, data: file("Screenshot") })
  const onUploaded = jest.fn()
  render(<UploadFileDialog isOpen onClose={jest.fn()} onUploaded={onUploaded} />)

  fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Screenshot" } })
  fireEvent.change(document.querySelector('input[type="file"]')!, {
    target: { files: [new File(["image bytes"], "screenshot.png", { type: "image/png" })] },
  })
  fireEvent.click(screen.getByRole("button", { name: "Upload image", exact: true }))
  await screen.findByText("The upload could not be confirmed. Retry to check the same upload.")

  expect(uploadFile).not.toHaveBeenCalled()
  expect(screen.getByLabelText("Name")).toBeEnabled()
  expect(screen.getByLabelText("Name")).toHaveValue("Screenshot")
  expect(document.querySelector('img[src^="data:image/png"]')).not.toBeNull()
  expect(screen.queryByRole("button", { name: "Retry upload" })).not.toBeInTheDocument()

  fireEvent.click(screen.getByRole("button", { name: "Upload image", exact: true }))
  await waitFor(() => expect(onUploaded).toHaveBeenCalledTimes(1))
  expect(newFileUploadID).toHaveBeenCalledTimes(2)
  expect(uploadFile).toHaveBeenCalledTimes(1)
})

test("a lost upload response retains the image and retries the same payload and receipt key", async () => {
  const upload = jest.mocked(uploadFile)
  upload.mockRejectedValueOnce(new RequestError("POST", "/api/admin/files", "transport", new Error("response lost")))
  upload.mockResolvedValue({ ok: true, data: file("Screenshot") })
  const onUploaded = jest.fn()
  render(<UploadFileDialog isOpen onClose={jest.fn()} onUploaded={onUploaded} />)
  fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Screenshot" } })
  fireEvent.change(document.querySelector('input[type="file"]')!, {
    target: { files: [new File(["image bytes"], "screenshot.png", { type: "image/png" })] },
  })
  fireEvent.click(screen.getByRole("button", { name: "Upload image", exact: true }))
  await screen.findByRole("button", { name: "Retry upload" })

  const original = upload.mock.calls[0][0]
  expect(original.submissionId).toBe("server-upload-1")
  expect(original.file.upload?.fileName).toBe("screenshot.png")
  expect(original.file.upload?.content).toBe(btoa("image bytes"))
  expect(screen.getByLabelText("Name")).toBeDisabled()
  expect(document.querySelector('img[src^="data:image/png"]')).not.toBeNull()

  fireEvent.click(screen.getByRole("button", { name: "Retry upload" }))
  await waitFor(() => expect(onUploaded).toHaveBeenCalledTimes(1))
  expect(upload.mock.calls[1][0]).toEqual(original)
  expect(newFileUploadID).toHaveBeenCalledTimes(1)
})

test("an unconfirmed upload can become a new editable operation without losing its image", async () => {
  const upload = jest.mocked(uploadFile)
  upload.mockRejectedValueOnce(new RequestError("POST", "/api/admin/files", "transport", new Error("response lost")))
  upload.mockResolvedValueOnce({ ok: false, status: 400, error: { errors: [{ field: "name", message: "Choose another name" }] } })
  render(<UploadFileDialog isOpen onClose={jest.fn()} onUploaded={jest.fn()} />)
  fireEvent.change(screen.getByLabelText("Name"), { target: { value: "First name" } })
  fireEvent.change(document.querySelector('input[type="file"]')!, {
    target: { files: [new File(["image bytes"], "screenshot.png", { type: "image/png" })] },
  })
  fireEvent.click(screen.getByRole("button", { name: "Upload image", exact: true }))
  await screen.findByRole("button", { name: "Edit as a new upload" })
  const original = upload.mock.calls[0][0]

  fireEvent.click(screen.getByRole("button", { name: "Edit as a new upload" }))
  fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Second name" } })
  fireEvent.click(screen.getByRole("button", { name: "Upload image", exact: true }))
  await screen.findByText("Choose another name")

  const replacement = upload.mock.calls[1][0]
  expect(replacement.submissionId).not.toBe(original.submissionId)
  expect(replacement.file).toEqual(original.file)
  expect(replacement.name).toBe("Second name")
  expect(screen.getByLabelText("Name")).toBeEnabled()
  expect(screen.getByLabelText("Name")).toHaveValue("Second name")
  expect(document.querySelector('img[src^="data:image/png"]')).not.toBeNull()
})

test("a partial deletion keeps failed files available and retries only those files", async () => {
  const remove = jest.mocked(deleteFiles)
  remove.mockResolvedValueOnce({ ok: true, data: {
    deleted: ["files/First"], pending: [], skipped: [], errors: [{ blobKey: "files/Second", message: "Storage unavailable" }],
  } })
  remove.mockResolvedValue({ ok: true, data: { deleted: ["files/Second"], pending: [], skipped: [], errors: [] } })
  const onChanged = jest.fn()
  const onClose = jest.fn()
  render(<DeleteFilesDialog files={[file("First"), file("Second")]} scope={protectedScope} onChanged={onChanged} onClose={onClose} />)
  fireEvent.click(screen.getByRole("button", { name: "Delete selected" }))
  await screen.findByText("Second: Storage unavailable")
  expect(onChanged).toHaveBeenCalledTimes(1)
  expect(onClose).not.toHaveBeenCalled()

  fireEvent.click(screen.getByRole("button", { name: "Delete selected" }))
  await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1))
  expect(remove.mock.calls[1]).toEqual([["files/Second"], { ...protectedScope, force: false }])
})

test("included deleted references do not require forced deletion", async () => {
  const remove = jest.mocked(deleteFiles)
  remove.mockResolvedValue({ ok: true, data: { deleted: ["files/Deleted post image"], pending: [], skipped: [], errors: [] } })
  const scope = { includeDeleted: true, includeDrafts: false }
  render(<DeleteFilesDialog files={[{ ...file("Deleted post image"), isInUse: true }]} scope={scope} onChanged={jest.fn()} onClose={jest.fn()} />)

  expect(screen.getByRole("checkbox")).not.toBeChecked()
  fireEvent.click(screen.getByRole("button", { name: "Delete selected" }))
  await waitFor(() => expect(remove).toHaveBeenCalledWith(["files/Deleted post image"], { ...scope, force: false }))
})

test("a selection larger than 100 retains completed progress when a later batch fails", async () => {
  const files = Array.from({ length: 150 }, (_, index) => file(`Image ${index + 1}`))
  const firstKeys = files.slice(0, 100).map(file => file.blobKey)
  const lastKeys = files.slice(100).map(file => file.blobKey)
  const remove = jest.mocked(deleteFiles)
  remove.mockResolvedValueOnce({
    ok: true,
    data: { deleted: firstKeys, pending: [], skipped: [], errors: [] },
  })
  remove.mockRejectedValueOnce(new RequestError("POST", "/api/admin/files/delete", "transport", new Error("offline")))
  remove.mockResolvedValueOnce({
    ok: true,
    data: { deleted: lastKeys, pending: [], skipped: [], errors: [] },
  })
  const onChanged = jest.fn()
  const onClose = jest.fn()
  render(<DeleteFilesDialog files={files} scope={protectedScope} onChanged={onChanged} onClose={onClose} />)

  fireEvent.click(screen.getByRole("button", { name: "Delete selected" }))
  await screen.findByText("Deletion could not be confirmed. Retry to check the remaining files.")
  expect(screen.getByText("100 deleted, 0 queued, 50 remaining")).toBeInTheDocument()
  expect(onChanged).toHaveBeenCalledWith(firstKeys, [])
  expect(remove.mock.calls.map(call => call[0].length)).toEqual([100, 50])

  fireEvent.click(screen.getByRole("button", { name: "Delete selected" }))
  await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1))
  expect(remove.mock.calls[2]).toEqual([lastKeys, { ...protectedScope, force: false }])
  expect(onChanged).toHaveBeenLastCalledWith(lastKeys, [])
})

test("cleanup resumes the same filter, cutoff and cursor after a transport failure", async () => {
  const prune = jest.mocked(pruneFiles)
  prune.mockResolvedValueOnce({ ok: true, data: {
    deleted: ["files/First"], pending: [], skipped: [], errors: [], nextCursor: "after-first",
  } })
  prune.mockRejectedValueOnce(new RequestError("POST", "/api/admin/files/prune", "transport", new Error("offline")))
  prune.mockResolvedValue({ ok: true, data: { deleted: [], pending: ["files/Second"], skipped: [], errors: [] } })
  const request = {
    search: "Screenshot", type: "files" as const, before: "2026-09-28T00:00:00Z",
    includeDeleted: true, includeDrafts: false,
  }
  render(<PruneFilesDialog request={request} count={2} onClose={jest.fn()} onChanged={jest.fn()} />)
  fireEvent.click(screen.getByRole("button", { name: "Delete matching unused images" }))
  await screen.findByRole("button", { name: "Continue cleanup" })
  expect(prune.mock.calls[1][0]).toEqual({ ...request, cursor: "after-first" })

  fireEvent.click(screen.getByRole("button", { name: "Continue cleanup" }))
  await screen.findByText(/1 deleted, 1 queued, 0 kept, 0 failed, complete/)
  expect(prune.mock.calls[2][0]).toEqual(prune.mock.calls[1][0])
})

test("a later HTTP rejection cannot turn an uncertain upload into a new operation", async () => {
  const upload = jest.mocked(uploadFile)
    .mockResolvedValueOnce({ ok: false, status: 403, unconfirmed: true, error: { errors: [{ message: "Access denied after retry" }] } })
    .mockResolvedValueOnce({ ok: false, status: 400, unconfirmed: false, error: { errors: [{ field: "name", message: "Validation changed" }] } })
    .mockResolvedValue({ ok: true, unconfirmed: false, data: file("Receipt image") })
  const onUploaded = jest.fn()
  render(<UploadFileDialog isOpen onClose={jest.fn()} onUploaded={onUploaded} />)
  fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Receipt image" } })
  fireEvent.change(document.querySelector('input[type="file"]')!, {
    target: { files: [new File(["image bytes"], "screenshot.png", { type: "image/png" })] },
  })
  fireEvent.click(screen.getByRole("button", { name: "Upload image", exact: true }))
  await screen.findByText("Access denied after retry")

  expect(screen.getByLabelText("Name")).toBeDisabled()
  fireEvent.click(screen.getByRole("button", { name: "Retry upload" }))
  await screen.findByText("Validation changed")
  expect(screen.getByLabelText("Name")).toBeDisabled()

  fireEvent.click(screen.getByRole("button", { name: "Retry upload" }))
  await waitFor(() => expect(onUploaded).toHaveBeenCalledTimes(1))
  expect(upload.mock.calls[1][0]).toEqual(upload.mock.calls[0][0])
  expect(upload.mock.calls[2][0]).toEqual(upload.mock.calls[0][0])
})
