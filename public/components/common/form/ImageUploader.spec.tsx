import React from "react"
import { jest, test, expect } from "@jest/globals"
import { render, fireEvent, waitFor } from "@testing-library/react"
import { ImageUploader } from "./ImageUploader"

test("saved preview follows publication while the editor stays mounted", () => {
  const view = render(<ImageUploader field="avatar" previewURL="/pending" onChange={jest.fn()} />)
  view.rerender(<ImageUploader field="avatar" previewURL="/approved" onChange={jest.fn()} />)
  expect(view.container.querySelector("img")?.getAttribute("src")).toBe("/approved")
})

test("updated saved preview does not replace a local upload or removal", async () => {
  const onChange = jest.fn()
  const view = render(<ImageUploader field="avatar" previewURL="/pending" onChange={onChange} />)
  fireEvent.change(view.container.querySelector('input[type="file"]')!, {
    target: { files: [new File(["draft"], "draft.png", { type: "image/png" })] },
  })
  await waitFor(() => expect(onChange).toHaveBeenCalled())
  const draft = view.container.querySelector("img")?.getAttribute("src")
  expect(draft).toMatch(/^data:image\/png;base64,/)
  view.rerender(<ImageUploader field="avatar" previewURL="/approved" onChange={onChange} />)
  expect(view.container.querySelector("img")?.getAttribute("src")).toBe(draft)
  fireEvent.click(view.getByText("X"))
  view.rerender(<ImageUploader field="avatar" previewURL="/new-approved" onChange={onChange} />)
  expect(view.container.querySelector("img")).toBeNull()
})
