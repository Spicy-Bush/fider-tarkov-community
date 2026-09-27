import React from "react"
import { afterEach, beforeEach, expect, test } from "@jest/globals"
import { cleanup, fireEvent, render } from "@testing-library/react"
import { ImageGallery } from "@fider/components/common/ImageGallery"
import { Fider } from "@fider/services"
import { PreviewPostModal } from "./PreviewPostModal"

beforeEach(() => {
  Fider.initialize({ settings: { assetsURL: "" }, tenant: { locale: "en" } })
  const modalRoot = document.createElement("div")
  modalRoot.id = "root-modal"
  document.body.append(modalRoot)
})

afterEach(() => {
  cleanup()
  document.getElementById("root-modal")?.remove()
})

test("preview shows draft replacements and saved images in order, excluding removals", () => {
  const view = render(
    <PreviewPostModal
      isOpen
      title="A draft with attachments"
      description="Draft description"
      attachments={[
        { bkey: "saved", remove: false },
        { bkey: "removed", remove: true },
        {
          bkey: "replaced",
          remove: false,
          upload: { contentType: "image/png", content: "ZHJhZnQ=" },
        },
        {
          remove: false,
          upload: { contentType: "image/jpeg", content: "bmV3" },
        },
      ]}
    />
  )

  const images = view.getByRole("dialog").querySelectorAll("img")
  expect(images).toHaveLength(3)
  expect(images[0]).toHaveAttribute("src", "/static/images/saved?size=100")
  expect(images[1]).toHaveAttribute("src", "data:image/png;base64,ZHJhZnQ=")
  expect(images[2]).toHaveAttribute("src", "data:image/jpeg;base64,bmV3")

  fireEvent.click(images[1])
  expect(document.body.lastElementChild?.querySelector("img"))
    .toHaveAttribute("src", "data:image/png;base64,ZHJhZnQ=")

  fireEvent.keyDown(window, { key: "ArrowLeft" })
  expect(document.body.lastElementChild?.querySelector("img"))
    .toHaveAttribute("src", "/static/images/saved?size=1500")
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
