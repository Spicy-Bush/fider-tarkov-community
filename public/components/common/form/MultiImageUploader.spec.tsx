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
  expect(view.queryByRole("button", { name: "Select image" })).toBeNull()
  expect(onChange).not.toHaveBeenCalled()

  fireEvent.click(view.getAllByRole("button", { name: "Remove image" })[0])

  expect(view.container.querySelectorAll("img")).toHaveLength(1)
  expect(view.getAllByRole("button", { name: "Select image" })).toHaveLength(1)
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
  expect(view.queryByRole("button", { name: "Select image" })).toBeNull()

  fireEvent.click(view.getByRole("button", { name: "Remove image" }))

  expect(view.container.querySelector("img")).toBeNull()
  expect(view.getAllByRole("button", { name: "Select image" })).toHaveLength(1)
  expect(onChange).toHaveBeenLastCalledWith([{ bkey: "removed-image", remove: true }])

  fireEvent.change(view.container.querySelector('input[type="file"]')!, {
    target: {
      files: [new File(["replacement"], "replacement.png", { type: "image/png" })],
    },
  })

  await waitFor(() => {
    expect(view.container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,cmVwbGFjZW1lbnQ=")
  })

  expect(view.queryByRole("button", { name: "Select image" })).toBeNull()

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
  expect(view.queryByRole("button", { name: "Remove image" })).toBeNull()
  expect(onChange).toHaveBeenCalledTimes(1)
  expect(onChange).toHaveBeenLastCalledWith([
    expect.objectContaining({ upload: expect.objectContaining({ content: "ZHJhZnQ=" }) }),
  ])

  view.rerender(
    <MultiImageUploader field="attachments" maxUploads={1} onChange={onChange} />
  )

  fireEvent.click(view.getByRole("button", { name: "Remove image" }))

  expect(onChange).toHaveBeenLastCalledWith([])
  expect(view.container.querySelector("img")).toBeNull()
  expect(view.getAllByRole("button", { name: "Select image" })).toHaveLength(1)
})

test("collecting attachments waits for the selected file and reserves its upload slot", async () => {
  let reader: FileReader

  jest.spyOn(FileReader.prototype, "readAsDataURL").mockImplementation(function (this: FileReader) {
    reader = this
  })

  const uploader = React.createRef<MultiImageUploader>()
  const view = render(<MultiImageUploader ref={uploader} field="attachments" maxUploads={1} />)

  fireEvent.change(view.container.querySelector('input[type="file"]')!, {
    target: {
      files: [new File(["draft"], "draft.png", { type: "image/png" })],
    },
  })

  const collected = jest.fn()
  const pending = uploader.current!.readUploads().then(collected)

  await act(async () => {})

  expect(collected).not.toHaveBeenCalled()
  expect(view.container.querySelectorAll('input[type="file"]')).toHaveLength(1)

  await act(async () => {
    Object.defineProperty(reader, "result", { value: "data:image/png;base64,ZHJhZnQ=" })
    reader.dispatchEvent(new Event("load"))
    await pending
  })

  expect(collected).toHaveBeenCalledWith([
    {
      bkey: undefined,
      remove: false,
      upload: {
        fileName: "draft.png",
        contentType: "image/png",
        content: "ZHJhZnQ=",
      },
    },
  ])
})

test("a failed file read can be retried or removed without silently submitting fewer attachments", async () => {
  const readers: FileReader[] = []

  jest.spyOn(FileReader.prototype, "readAsDataURL").mockImplementation(function (this: FileReader) {
    readers.push(this)
  })

  const uploader = React.createRef<MultiImageUploader>()
  const view = render(<MultiImageUploader ref={uploader} field="attachments" maxUploads={1} />)

  fireEvent.change(view.container.querySelector('input[type="file"]')!, {
    target: {
      files: [new File(["draft"], "draft.png", { type: "image/png" })],
    },
  })

  await act(async () => {
    readers[0].dispatchEvent(new Event("error"))
  })

  await expect(uploader.current!.readUploads()).resolves.toBeUndefined()
  expect(view.getByRole("alert")).toHaveTextContent("Could not read draft.png.")

  fireEvent.click(view.getByRole("button", { name: "Retry image" }))

  await act(async () => {
    Object.defineProperty(readers[1], "result", { value: "data:image/png;base64,ZHJhZnQ=" })
    readers[1].dispatchEvent(new Event("load"))
  })

  await expect(uploader.current!.readUploads()).resolves.toEqual([
    expect.objectContaining({ upload: expect.objectContaining({ content: "ZHJhZnQ=" }) }),
  ])

  fireEvent.click(view.getByRole("button", { name: "Remove image" }))
  fireEvent.change(view.container.querySelector('input[type="file"]')!, {
    target: {
      files: [new File(["second"], "second.png", { type: "image/png" })],
    },
  })

  await act(async () => {
    readers[2].dispatchEvent(new Event("error"))
  })

  fireEvent.click(view.getByRole("button", { name: "Remove image" }))

  await expect(uploader.current!.readUploads()).resolves.toEqual([])
})

test("a superseded file read cannot replace the newer selected image", async () => {
  const readers: FileReader[] = []

  jest.spyOn(FileReader.prototype, "readAsDataURL").mockImplementation(function (this: FileReader) {
    readers.push(this)
  })

  const uploader = React.createRef<MultiImageUploader>()
  const onChange = jest.fn()
  const view = render(
    <MultiImageUploader ref={uploader} field="attachments" maxUploads={1} onChange={onChange} />
  )
  const input = view.container.querySelector('input[type="file"]')!

  for (const name of ["old", "new"]) {
    fireEvent.change(input, {
      target: {
        files: [new File([name], `${name}.png`, { type: "image/png" })],
      },
    })
  }

  await act(async () => {
    Object.defineProperty(readers[1], "result", { value: "data:image/png;base64,bmV3" })
    readers[1].dispatchEvent(new Event("load"))
    Object.defineProperty(readers[0], "result", { value: "data:image/png;base64,b2xk" })
    readers[0].dispatchEvent(new Event("load"))
  })

  expect(onChange).toHaveBeenCalledTimes(1)
  expect(view.container.querySelector("img")).toHaveAttribute("src", "data:image/png;base64,bmV3")
  await expect(uploader.current!.readUploads()).resolves.toEqual([
    expect.objectContaining({ upload: expect.objectContaining({ fileName: "new.png", content: "bmV3" }) }),
  ])
})
