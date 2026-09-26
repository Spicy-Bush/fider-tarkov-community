import React from "react"
import { afterEach, beforeEach, expect, jest, test } from "@jest/globals"
import { act, fireEvent, render, waitFor } from "@testing-library/react"
import { MultiImageUploader } from "./MultiImageUploader"
import { ImageUpload } from "@fider/models"
import { Fider } from "@fider/services"

beforeEach(() => {
  Fider.initialize({ settings: { assetsURL: "" }, tenant: {} })
})

afterEach(() => {
  jest.restoreAllMocks()
})

test("existing attachments fill the limit and removal sends only the changed attachment", () => {
  const onChange = jest.fn()
  const view = render(
    <MultiImageUploader
      field="attachments"
      bkeys={["first-image", "second-image"]}
      maxUploads={2}
      onChange={onChange}
    />
  )

  expect(view.container.querySelectorAll("img")).toHaveLength(2)
  expect(view.getAllByRole("button")).toHaveLength(2)
  expect(onChange).not.toHaveBeenCalled()

  fireEvent.click(view.getAllByRole("button", { name: "X" })[0])

  expect(view.container.querySelectorAll("img")).toHaveLength(1)
  expect(view.getAllByRole("button")).toHaveLength(2)
  expect(onChange).toHaveBeenLastCalledWith([
    expect.objectContaining({ bkey: "first-image", remove: true }),
  ])
})

test("restored uploads and removals survive replacement without duplicate upload controls", async () => {
  const onChange = jest.fn()
  const saved: ImageUpload[] = [
    {
      remove: false,
      upload: {
        fileName: "draft.png",
        contentType: "image/png",
        content: "ZHJhZnQ=",
      },
    },
    { bkey: "removed-image", remove: true },
  ]

  const view = render(
    <MultiImageUploader
      field="attachments"
      initialUploads={saved}
      maxUploads={1}
      onChange={onChange}
    />
  )

  expect(view.container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,ZHJhZnQ=")
  expect(view.getAllByRole("button")).toHaveLength(1)

  fireEvent.click(view.getByRole("button", { name: "X" }))

  expect(view.container.querySelector("img")).toBeNull()
  expect(view.getAllByRole("button")).toHaveLength(1)
  expect(onChange).toHaveBeenLastCalledWith([{ bkey: "removed-image", remove: true }])

  fireEvent.change(view.container.querySelector('input[type="file"]')!, {
    target: {
      files: [new File(["replacement"], "replacement.png", { type: "image/png" })],
    },
  })

  await waitFor(() => {
    expect(view.container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,cmVwbGFjZW1lbnQ=")
  })

  expect(view.getAllByRole("button")).toHaveLength(1)

  const changes = onChange.mock.calls[onChange.mock.calls.length - 1][0]
  expect(changes).toHaveLength(2)
  expect(changes).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        remove: false,
        upload: {
          fileName: "replacement.png",
          contentType: "image/png",
          content: "cmVwbGFjZW1lbnQ=",
        },
      }),
      { bkey: "removed-image", remove: true },
    ])
  )
})

test("an in-progress file read remains reflected in the parent when controls become disabled", async () => {
  const readers: FileReader[] = []
  jest.spyOn(FileReader.prototype, "readAsDataURL").mockImplementation(function (this: FileReader) {
    readers.push(this)
  })

  const onChange = jest.fn()
  const view = render(
    <MultiImageUploader field="attachments" maxUploads={1} onChange={onChange} />
  )

  fireEvent.change(view.container.querySelector('input[type="file"]')!, {
    target: {
      files: [new File(["draft"], "draft.png", { type: "image/png" })],
    },
  })

  view.rerender(
    <MultiImageUploader field="attachments" maxUploads={1} onChange={onChange} disabled />
  )

  await act(async () => {
    Object.defineProperty(readers[0], "result", { value: "data:image/png;base64,ZHJhZnQ=" })
    readers[0].dispatchEvent(new Event("load"))
  })

  expect(view.container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,ZHJhZnQ=")
  expect(view.queryByRole("button", { name: "X" })).toBeNull()
  expect(onChange).toHaveBeenCalledTimes(1)
  expect(onChange).toHaveBeenLastCalledWith([
    expect.objectContaining({ upload: expect.objectContaining({ content: "ZHJhZnQ=" }) }),
  ])

  view.rerender(
    <MultiImageUploader field="attachments" maxUploads={1} onChange={onChange} />
  )

  fireEvent.click(view.getByRole("button", { name: "X" }))

  expect(onChange).toHaveBeenLastCalledWith([])
  expect(view.container.querySelector("img")).toBeNull()
  expect(view.getAllByRole("button")).toHaveLength(1)
})
