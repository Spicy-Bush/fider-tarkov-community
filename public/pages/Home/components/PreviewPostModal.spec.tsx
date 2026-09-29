import React from "react"
import { afterEach, beforeEach, expect, test } from "@jest/globals"
import { cleanup, fireEvent, render } from "@testing-library/react"
import { ImageGallery } from "@fider/components/common/ImageGallery"
import { Fider } from "@fider/services"
import { PreviewPostModal } from "./PreviewPostModal"

beforeEach(() => {
  Fider.initialize({ settings: { assetsURL: "" }, tenant: { locale: "en" } })
  URL.createObjectURL = jest.fn(file => `blob:${(file as File).name}`)
  URL.revokeObjectURL = jest.fn()

  const modalRoot = document.createElement("div")
  modalRoot.id = "root-modal"
  document.body.append(modalRoot)
})

afterEach(() => {
  cleanup()
  document.getElementById("root-modal")?.remove()
})

test("preview shows draft replacements and saved images in order, excluding removals", () => {
  const replacement = new File(["draft"], "replacement.png", { type: "image/png" })
  const added = new File(["new"], "new.jpg", { type: "image/jpeg" })
  const view = render(
    <PreviewPostModal
      isOpen
      title="A draft with attachments"
      description="Draft description"
      attachments={[
        { kind: "stored", bkey: "saved" },
        { kind: "removed", bkey: "removed" },
        { kind: "local", fileId: "replacement", file: replacement, replaces: "replaced" },
        { kind: "missing", fileId: "missing", fileName: "missing.png" },
        { kind: "local", fileId: "new", file: added },
      ]}
    />
  )

  const images = view.getByRole("dialog").querySelectorAll("img")
  expect(images).toHaveLength(3)
  expect(images[0]).toHaveAttribute("src", "/static/images/saved?size=200")
  expect(images[1]).toHaveAttribute("src", "blob:replacement.png")
  expect(images[2]).toHaveAttribute("src", "blob:new.jpg")
  expect(URL.createObjectURL).toHaveBeenCalledWith(replacement)
  expect(URL.createObjectURL).toHaveBeenCalledWith(added)

  fireEvent.click(images[1])
  expect(document.body.lastElementChild?.querySelector("img"))
    .toHaveAttribute("src", "blob:replacement.png")

  fireEvent.keyDown(window, { key: "ArrowLeft" })
  expect(document.body.lastElementChild?.querySelector("img"))
    .toHaveAttribute("src", "/static/images/saved?size=1500")

  view.unmount()
  expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:replacement.png")
  expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:new.jpg")
})

test("published galleries retain sized thumbnails and full-size viewing", () => {
  const view = render(<ImageGallery bkeys={["published"]} />)
  const image = view.container.querySelector("img")!
  expect(image).toHaveAttribute("src", "/static/images/published?size=200")

  fireEvent.click(image)
  expect(document.body.lastElementChild?.querySelector("img"))
    .toHaveAttribute("src", "/static/images/published?size=1500")
})

test("moving between duplicate images keeps the loaded image visible", () => {
  const view = render(<ImageGallery bkeys={["same", "same"]} />)
  fireEvent.click(view.container.querySelector("img")!)

  const enlarged = document.body.lastElementChild?.querySelector("img")!
  fireEvent.load(enlarged)
  expect(enlarged).toHaveClass("opacity-100")

  fireEvent.keyDown(window, { key: "ArrowRight" })
  expect(enlarged).toHaveClass("opacity-100")
  expect(view.getByText("2 / 2")).toBeInTheDocument()
})
