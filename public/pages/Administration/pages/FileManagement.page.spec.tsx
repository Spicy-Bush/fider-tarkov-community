import React from "react"
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import FileManagementPage from "./FileManagement.page"
import { deleteFiles, FileInfo, FileLibraryOptions, FileListResponse, listFiles, pruneFiles, refreshFileInventory } from "@fider/services/actions/file"
import { RequestError } from "@fider/services/http"

jest.mock("@fider/services/actions/file", () => ({
  ...jest.requireActual("@fider/services/actions/file"),
  listFiles: jest.fn(),
  refreshFileInventory: jest.fn(),
  deleteFiles: jest.fn(),
  pruneFiles: jest.fn(),
}))

const list = jest.mocked(listFiles)
const options: FileLibraryOptions = {
  defaults: {
    page: 1, pageSize: 20, search: "", type: "all", usage: "all",
    includeDeleted: false, includeDrafts: false, sortBy: "createdAt", sortDir: "desc",
  },
  types: [{ value: "all", label: "All types" }, { value: "files", label: "Admin uploads" }],
  usage: [{ value: "all", label: "All usage" }, { value: "used", label: "In use" }, { value: "unused", label: "Unused" }],
  sort: [{ value: "createdAt", label: "Date uploaded" }, { value: "name", label: "Name" }, { value: "size", label: "Size" }],
  pageSizes: [20, 50, 100],
  maxPage: 1000000000,
  maxPageSize: 100,
  maxImageBytes: 50_000 * 1024,
}
const file = (name: string): FileInfo => ({
  name, blobKey: `files/${name}`, size: 1234, contentType: "image/png", createdAt: "2026-09-28T00:00:00Z",
  width: 200, height: 200, thumbnailURL: `/thumb/${name}`, url: `/image/${name}`, isInUse: false, hasProtectedReferences: false, state: "ready",
})
const collection = (files: FileInfo[], changes: Partial<FileListResponse> = {}): FileListResponse => ({
  files, total: files.length, page: 1, pageSize: 20, totalPages: 1,
  listedAt: "2026-09-28T03:45:00Z",
  inventory: { state: "ready", scanned: files.length, skipped: 0 }, ...changes,
})

beforeEach(() => {
  list.mockReset()
  list.mockResolvedValue({ ok: true, data: collection([file("Original")]) })
  jest.mocked(refreshFileInventory).mockReset().mockResolvedValue({ ok: true, data: undefined })
  history.replaceState({}, "", "/admin/files")
  HTMLElement.prototype.scrollIntoView = jest.fn()
  const modal = document.createElement("div")
  modal.id = "root-modal"
  document.body.appendChild(modal)
})

afterEach(() => document.getElementById("root-modal")?.remove())

test("search requests the current text and clearing an empty result reloads all images", async () => {
  list.mockImplementation(async query => ({ ok: true, data: collection(query.search ? [] : [file("Original")]) }))
  render(<FileManagementPage options={options} />)
  await screen.findByText("Original")
  expect(list).toHaveBeenCalledTimes(1)

  fireEvent.change(screen.getByLabelText("Search images"), { target: { value: "no matching image" } })
  await screen.findByText("No images found")
  expect(list.mock.calls[1][0].search).toBe("no matching image")

  fireEvent.click(screen.getAllByRole("button", { name: "Clear filters" })[0])
  await screen.findByText("Original")
  expect(list.mock.calls[2][0].search).toBe("")
  expect(screen.getByLabelText("Search images")).toHaveValue("")
})

test("a superseded sort response cannot replace the current list", async () => {
  let resolveOld!: (result: Awaited<ReturnType<typeof listFiles>>) => void
  list.mockImplementation(query => {
    if (query.sortBy === "name") return new Promise(resolve => { resolveOld = resolve })
    return Promise.resolve({ ok: true, data: collection([file(query.sortBy === "size" ? "Size result" : "Original")]) })
  })
  render(<FileManagementPage options={options} />)
  await screen.findByText("Original")

  fireEvent.change(screen.getByLabelText("Sort by"), { target: { value: "name" } })
  const oldSignal = list.mock.calls[1][1]
  fireEvent.change(screen.getByLabelText("Sort by"), { target: { value: "size" } })
  await screen.findByText("Size result")
  expect(oldSignal.aborted).toBe(true)

  await act(async () => resolveOld({ ok: true, data: collection([file("Old name result")]) }))
  expect(screen.queryByText("Old name result")).not.toBeInTheDocument()
  expect(screen.getByText("Size result")).toBeInTheDocument()
})

test("transport failure ends loading and the same query can be retried", async () => {
  list.mockRejectedValueOnce(new RequestError("GET", "/api/admin/files", "transport", new Error("offline")))
  render(<FileManagementPage options={options} />)
  await screen.findByText("Images could not be loaded. Check your connection and try again.")
  expect(screen.queryByText("Loading images")).not.toBeInTheDocument()

  fireEvent.click(screen.getByRole("button", { name: "Retry loading images" }))
  await screen.findByText("Original")
  expect(list.mock.calls[1][0]).toEqual(list.mock.calls[0][0])
})

test("pagination follows the server's corrected page after a collection shrinks", async () => {
  list.mockResolvedValueOnce({ ok: true, data: collection([file("First page")], { total: 21, totalPages: 2 }) })
  list.mockResolvedValue({ ok: true, data: collection([file("Remaining image")], { total: 20 }) })
  render(<FileManagementPage options={options} />)
  await screen.findByText("First page")
  fireEvent.click(screen.getAllByRole("button", { name: "Next page" })[0])
  await screen.findByText("Remaining image")

  expect(list.mock.calls[1][0].page).toBe(2)
  expect(screen.getByText("1-20 of 20 images")).toBeInTheDocument()
  expect(screen.queryByText("No images found")).not.toBeInTheDocument()
})

test("selection survives an empty filtered collection and can be reviewed independently", async () => {
  render(<FileManagementPage options={options} />)
  await screen.findByText("Original")
  fireEvent.click(screen.getByRole("checkbox", { name: "Select Original" }))
  expect(screen.getByText("1 selected")).toBeInTheDocument()

  list.mockResolvedValue({ ok: true, data: collection([]) })
  fireEvent.change(screen.getByLabelText("Usage"), { target: { value: "unused" } })
  await screen.findByText("No images found")
  expect(screen.getByText("1 selected")).toBeInTheDocument()
  fireEvent.click(screen.getByRole("button", { name: "Review selected" }))
  const review = screen.getByRole("dialog")
  expect(within(review).getByText("Original")).toBeInTheDocument()
  fireEvent.click(within(review).getByRole("checkbox", { name: "Keep Original selected" }))
  expect(within(review).getByText("No images selected.")).toBeInTheDocument()
  expect(within(review).getByRole("button", { name: "Delete selected" })).toBeDisabled()
})

test("preview choices survive page changes and deletion keeps skipped selections", async () => {
  const first = file("First")
  const second = file("Second")
  const third = file("Third")
  list.mockImplementation(async query => ({
    ok: true,
    data: collection(query.page === 1 ? [first, second] : [third], {
      page: query.page,
      total: 21,
      totalPages: 2,
    }),
  }))
  jest.mocked(deleteFiles).mockResolvedValue({
    ok: true,
    data: { deleted: [first.blobKey], pending: [], skipped: [third.blobKey], errors: [] },
  })
  render(<FileManagementPage options={options} />)
  fireEvent.click(await screen.findByRole("button", { name: "Preview First" }))
  const preview = within(screen.getByRole("dialog"))
  expect(preview.getByRole("img")).toHaveAttribute("src", "/api/admin/files/thumbnail?key=files%2FFirst&size=512")
  expect(preview.getByRole("link", { name: "Download" })).toHaveAttribute("href", "/api/admin/files/download?key=files%2FFirst")
  expect(preview.getByRole("link", { name: "Open full-size image" })).toHaveAttribute("href", "/api/admin/files/download?key=files%2FFirst&inline=true")
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("checkbox", { name: "Select image" }))
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Next image" }))
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("checkbox", { name: "Select image" }))
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Continue browsing" }))

  fireEvent.click(screen.getAllByRole("button", { name: "Page 2" })[0])
  await screen.findByRole("checkbox", { name: "Select Third" })
  fireEvent.click(screen.getByRole("button", { name: "Select this page" }))
  expect(screen.getByText("3 selected")).toBeInTheDocument()
  fireEvent.click(screen.getByRole("button", { name: "Select this page" }))
  expect(screen.getByText("2 selected")).toBeInTheDocument()
  fireEvent.click(screen.getByRole("checkbox", { name: "Select Third" }))

  fireEvent.click(screen.getByRole("button", { name: "Review selected" }))
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("checkbox", { name: "Keep Second selected" }))
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete selected" }))
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete selected" }))
  await screen.findByText("Third is in use and was kept.")

  expect(deleteFiles).toHaveBeenCalledWith([first.blobKey, third.blobKey], {
    force: false, includeDeleted: false, includeDrafts: false,
  })
  expect(screen.getByText("1 selected")).toBeInTheDocument()
  expect(list.mock.calls.at(-1)![0].page).toBe(2)
})

test("cleanup scopes protect deleted content and drafts until explicitly included", async () => {
  render(<FileManagementPage options={options} />)
  await screen.findByText("Original")
  expect(list.mock.calls[0][0]).toMatchObject({ includeDeleted: false, includeDrafts: false })

  fireEvent.click(screen.getByRole("button", { name: "Review unused" }))
  await screen.findByText("Original")
  fireEvent.click(screen.getByRole("checkbox", { name: "Allow cleanup of images in deleted content" }))
  await screen.findByText("Original")
  expect(list.mock.calls.at(-1)![0]).toMatchObject({ usage: "unused", includeDeleted: true, includeDrafts: false })

  fireEvent.click(screen.getByRole("button", { name: "Clear filters" }))
  await screen.findByText("Original")
  expect(list.mock.calls.at(-1)![0]).toMatchObject({ usage: "all", includeDeleted: false, includeDrafts: false })
})

test("pending discovery is not presented as a complete empty inventory", async () => {
  list.mockResolvedValue({ ok: true, data: collection([], { inventory: { state: "pending", scanned: 12, skipped: 0 } }) })
  render(<FileManagementPage options={options} />)
  await screen.findByText("Discovering images 12 checked.")
  expect(screen.queryByText("No images found")).not.toBeInTheDocument()

  list.mockResolvedValue({ ok: true, data: collection([file("Discovered image")]) })
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }))
  await screen.findByText("Discovered image")
  expect(refreshFileInventory).toHaveBeenCalledTimes(1)
  expect(screen.queryByText(/Discovering images/)).not.toBeInTheDocument()
})

test("cleanup uses the timestamp of the reviewed server listing", async () => {
  history.replaceState({}, "", "/admin/files?usage=unused")
  jest.mocked(pruneFiles).mockResolvedValue({
    ok: true,
    data: { deleted: ["files/Original"], pending: [], skipped: [], errors: [], nextCursor: "" },
  })

  render(<FileManagementPage options={options} />)
  fireEvent.click(await screen.findByRole("button", { name: "Review all 1 matching unused images" }))
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete matching unused images" }))

  await waitFor(() => expect(pruneFiles).toHaveBeenCalledWith(expect.objectContaining({
    before: "2026-09-28T03:45:00Z",
    includeDeleted: false,
    includeDrafts: false,
  })))
})

test("invalid storage names are reported while usable files remain selectable", async () => {
  list.mockResolvedValue({
    ok: true,
    data: collection([file("Original")], { inventory: { state: "ready", scanned: 3, skipped: 2 } }),
  })

  render(<FileManagementPage options={options} />)
  await screen.findByText("2 stored files have invalid names and were skipped.")
  fireEvent.click(screen.getByRole("checkbox", { name: "Select Original" }))
  expect(screen.getByRole("button", { name: "Delete selected" })).toBeEnabled()
})

test("background discovery keeps the same row, keyboard focus and selection", async () => {
  jest.useFakeTimers()
  try {
    list.mockResolvedValueOnce({ ok: true, data: collection([file("Original")], { inventory: { state: "pending", scanned: 1, skipped: 0 } }) })
    list.mockResolvedValue({ ok: true, data: collection([file("Original"), file("Discovered image")]) })
    render(<FileManagementPage options={options} />)
    const checkbox = await screen.findByRole("checkbox", { name: "Select Original" })
    checkbox.focus()
    fireEvent.click(checkbox)

    await act(async () => jest.advanceTimersByTime(5000))
    expect(list).toHaveBeenCalledTimes(2)
    expect(screen.getByText("Discovered image")).toBeInTheDocument()
    expect(screen.getByRole("checkbox", { name: "Select Original" })).toBe(checkbox)
    expect(checkbox).toBeChecked()
    expect(checkbox).toHaveFocus()
  } finally {
    jest.useRealTimers()
  }
})

test("oversized legacy images remain manageable without preview or download requests", async () => {
  list.mockResolvedValue({ ok: true, data: collection([{ ...file("Large archive image"), size: 50_000 * 1024 + 1 }]) })
  render(<FileManagementPage options={options} />)
  await screen.findByText("Large archive image")

  expect(screen.getByRole("button", { name: "Preview Large archive image" })).toBeDisabled()
  expect(document.querySelector('img[src="/thumb/Large archive image"]')).toBeNull()
  expect(screen.queryByRole("link", { name: "Download" })).not.toBeInTheDocument()
  expect(screen.getByText(/Use a backup export/)).toBeInTheDocument()
  expect(screen.getByRole("button", { name: "Delete", exact: true })).toBeEnabled()
})

test("all-matching cleanup waits for a complete inventory while selected deletion remains available", async () => {
  history.replaceState({}, "", "/admin/files?usage=unused")
  list.mockResolvedValue({ ok: true, data: collection([file("Original")], { inventory: { state: "pending", scanned: 1, skipped: 0 } }) })
  render(<FileManagementPage options={options} />)
  await screen.findByText("Original")

  expect(screen.getByRole("button", { name: "Review all 1 matching unused images" })).toBeDisabled()
  fireEvent.click(screen.getByRole("checkbox", { name: "Select Original" }))
  expect(screen.getByRole("button", { name: "Delete selected" })).toBeEnabled()
})

test("server options supply filter choices and initial values", async () => {
  const supplied: FileLibraryOptions = {
    ...options,
    defaults: { ...options.defaults, pageSize: 50, type: "files", sortBy: "name", sortDir: "asc" },
    types: [{ value: "files", label: "Uploaded files" }],
    pageSizes: [50, 100],
    maxImageBytes: 1000,
  }
  render(<FileManagementPage options={supplied} />)
  await screen.findByText("Original")

  expect(list.mock.calls[0][0]).toEqual(supplied.defaults)
  expect(screen.getByRole("option", { name: "Uploaded files" })).toBeInTheDocument()
  expect(screen.queryByRole("option", { name: "All types" })).not.toBeInTheDocument()
  expect(screen.getByLabelText("Per page")).toHaveValue("50")
  expect(screen.getByText("No preview")).toBeInTheDocument()
  expect(screen.queryByRole("link", { name: "Download" })).not.toBeInTheDocument()
})
