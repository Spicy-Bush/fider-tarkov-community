import React from "react"
import { act, fireEvent, render } from "@testing-library/react"
import { Fider } from "@fider/services"
import { CommentAttachments } from "./CommentAttachments"

beforeEach(() => {
  Fider.initialize({ settings: { assetsURL: "" }, tenant: {} })
})

test("editing retains existing images and removal stays removed across renders", async () => {
  const onChange = jest.fn()
  const props = {
    existing: ["first", "second"],
    allowUploads: true,
    maxUploads: 2,
    disabled: false,
    onChange,
  }
  const view = render(<CommentAttachments {...props} attachments={[]} />)
  expect(view.getAllByRole("button", { name: "Preview image" })).toHaveLength(2)
  expect(view.queryByRole("button", { name: "Select image" })).toBeNull()

  await act(async () => {
    fireEvent.click(view.getAllByRole("button", { name: "Remove image" })[0])
  })
  expect(onChange).toHaveBeenCalledWith([
    { bkey: "second", kind: "stored" },
    { bkey: "first", kind: "removed" },
  ])

  view.rerender(<CommentAttachments {...props} attachments={onChange.mock.calls[0][0]} />)
  expect(view.getAllByRole("button", { name: "Preview image" })).toHaveLength(1)
  expect(view.getByRole("button", { name: "Select image" })).toBeEnabled()

  view.rerender(<CommentAttachments {...props} allowUploads={false} attachments={onChange.mock.calls[0][0]} />)
  expect(view.queryByRole("button", { name: "Select image" })).toBeNull()
  expect(view.getByRole("button", { name: "Remove image" })).toBeEnabled()
})
