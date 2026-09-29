import React, { useState } from "react"
import { act, fireEvent, render } from "@testing-library/react"
import { MultiImageUploader } from "./MultiImageUploader"
import { DraftImage } from "@fider/services/draftImages"
import { Fider } from "@fider/services"

jest.mock("@fider/services/postSubmission", () => {
  let id = 0
  return { newSubmissionID: () => `selected-${++id}` }
})

beforeEach(() => {
  Fider.initialize({ settings: { assetsURL: "" }, tenant: {} })
  URL.createObjectURL = jest.fn(() => "blob:selected")
  URL.revokeObjectURL = jest.fn()
})

function Editor({ initial, maxUploads = 3 }: { initial: DraftImage[]; maxUploads?: number }) {
  const [value, setValue] = useState(initial)
  return <MultiImageUploader field="attachments" value={value} maxUploads={maxUploads} onChange={setValue} />
}

test("existing images fill the limit and removing one publishes a tombstone while retaining the others", async () => {
  const onChange = jest.fn()
  const value = [{ bkey: "first", kind: "stored" }, { bkey: "second", kind: "stored" }]
  const view = render(<MultiImageUploader field="attachments" value={value} maxUploads={2} onChange={onChange} />)
  expect(view.queryByRole("button", { name: "Select image" })).toBeNull()
  await act(async () => { fireEvent.click(view.getAllByRole("button", { name: "Remove image" })[0]) })
  expect(onChange).toHaveBeenCalledWith([{ bkey: "second", kind: "stored" }, { bkey: "first", kind: "removed" }])
})

test("selecting a file immediately gives its bytes to the parent without starting a FileReader", () => {
  const read = jest.spyOn(FileReader.prototype, "readAsDataURL")
  const onChange = jest.fn()
  const file = new File(["image bytes"], "draft.png", { type: "image/png" })
  const view = render(<MultiImageUploader field="attachments" value={[]} maxUploads={2} onChange={onChange} />)
  fireEvent.change(view.container.querySelector('input[type="file"]')!, { target: { files: [file] } })
  expect(onChange).toHaveBeenCalledWith([expect.objectContaining({ kind: "local", file })])
  expect(read).not.toHaveBeenCalled()
  read.mockRestore()
})

test("two selected files survive normalization of a separate image and an external removal", async () => {
  const first: DraftImage = { kind: "local", fileId: "first", file: new File(["one"], "first.png") }
  const second: DraftImage = { kind: "local", fileId: "second", file: new File(["two"], "second.png") }
  const previous: DraftImage = { kind: "local", fileId: "previous", file: new File(["x"], "previous.png", { type: "image/png" }) }
  const onChange = jest.fn()
  const view = render(<MultiImageUploader field="attachments" value={[previous, first, second]} maxUploads={3} onChange={onChange} />)
  const normalized = { bkey: "attachments/saved", kind: "stored" }
  view.rerender(<MultiImageUploader field="attachments" value={[normalized, first, second]} maxUploads={3} onChange={onChange} />)
  expect(view.container.querySelectorAll("img")).toHaveLength(3)
  view.rerender(<MultiImageUploader field="attachments" value={[normalized, second]} maxUploads={3} onChange={onChange} />)
  expect(view.container.querySelectorAll("img")).toHaveLength(2)
  expect(view.getByRole("button", { name: "Select image" })).toBeVisible()
  await act(async () => { fireEvent.click(view.getAllByRole("button", { name: "Remove image" })[0]) })
  expect(onChange).toHaveBeenLastCalledWith([second, { bkey: "attachments/saved", kind: "removed" }])
})

test("a newly selected image reserves its slot and can be removed without any asynchronous read", async () => {
  const view = render(<Editor initial={[]} maxUploads={1} />)
  fireEvent.change(view.container.querySelector('input[type="file"]')!, { target: { files: [new File(["image"], "draft.png")] } })
  expect(view.queryByRole("button", { name: "Select image" })).toBeNull()
  expect(view.getByRole("button", { name: "Preview image" })).toBeVisible()
  await act(async () => { fireEvent.click(view.getByRole("button", { name: "Remove image" })) })
  expect(view.getByRole("button", { name: "Select image" })).toBeVisible()
  expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:selected")
})

test("disabled images retain previews and expose no removal or upload action", () => {
  const view = render(<MultiImageUploader field="attachments" value={[{ bkey: "saved", kind: "stored" }]} maxUploads={2} disabled onChange={jest.fn()} />)
  expect(view.getByRole("button", { name: "Preview image" })).toBeVisible()
  expect(view.queryByRole("button", { name: "Remove image" })).toBeNull()
  expect(view.getByRole("button", { name: "Select image" })).toBeDisabled()
})

test("an unavailable image keeps its name and can be removed or replaced without an extra slot", async () => {
  const missing = { kind: "missing", fileId: "missing", fileName: "Unavailable.png" }
  const view = render(<Editor initial={[missing]} maxUploads={1} />)
  expect(view.getByRole("alert")).toHaveTextContent("Unavailable.png is unavailable")
  expect(view.getByRole("button", { name: "Remove image" })).toBeEnabled()
  expect(view.getByRole("button", { name: "Select image" })).toBeEnabled()

  fireEvent.change(view.container.querySelector('input[type="file"]')!, {
    target: { files: [new File(["replacement"], "Replacement.png", { type: "image/png" })] },
  })
  expect(view.queryByRole("alert")).toBeNull()
  expect(view.getByRole("button", { name: "Preview image" })).toBeVisible()
  expect(view.queryByRole("button", { name: "Select image" })).toBeNull()

  await act(async () => { fireEvent.click(view.getByRole("button", { name: "Remove image" })) })
  expect(view.getByRole("button", { name: "Select image" })).toBeVisible()
})
