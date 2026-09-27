import React from "react"
import { afterEach, beforeEach, expect, jest, test } from "@jest/globals"
import { act, fireEvent, render } from "@testing-library/react"
import { ImagePicker } from "./ImagePicker"

const originalCreateURL = URL.createObjectURL
const originalRevokeURL = URL.revokeObjectURL

beforeEach(() => {
  URL.createObjectURL = jest.fn((file: Blob) => `blob:${file.size}`)
  URL.revokeObjectURL = jest.fn()
})

afterEach(() => {
  URL.createObjectURL = originalCreateURL
  URL.revokeObjectURL = originalRevokeURL
  jest.restoreAllMocks()
})

test("selecting returns the original File without encoding it", () => {
  const onSelect = jest.fn()
  const read = jest.spyOn(FileReader.prototype, "readAsDataURL")
  const file = new File(["draft"], "draft.png", { type: "image/png" })
  const view = render(<ImagePicker field="attachment" onSelect={onSelect} />)

  fireEvent.change(view.container.querySelector('input[type="file"]')!, { target: { files: [file] } })

  expect(onSelect).toHaveBeenCalledWith(file)
  expect(read).not.toHaveBeenCalled()
})

test("the displayed File owns its object URL through replacement and unmount", async () => {
  const first = new File(["one"], "first.png", { type: "image/png" })
  const second = new File(["second"], "second.png", { type: "image/png" })
  const remove = jest.fn()
  const view = render(<ImagePicker field="attachment" image={first} onRemove={remove} />)

  expect(view.container.querySelector("img")).toHaveAttribute("src", "blob:3")
  view.rerender(<ImagePicker field="attachment" image={second} onRemove={remove} />)

  expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:3")
  expect(view.container.querySelector("img")).toHaveAttribute("src", "blob:6")
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Remove image" }))
  })
  expect(remove).toHaveBeenCalledWith()

  view.unmount()
  expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:6")
  expect(URL.revokeObjectURL).toHaveBeenCalledTimes(2)
})

test("a disabled preview keeps its image and offers no removal", () => {
  const view = render(<ImagePicker field="attachment" image="/saved-image" disabled onRemove={jest.fn()} />)

  expect(view.container.querySelector("img")).toHaveAttribute("src", "/saved-image")
  expect(view.queryByRole("button", { name: "Remove image" })).toBeNull()
  expect(view.container.querySelector('input[type="file"]')).toBeDisabled()
})
