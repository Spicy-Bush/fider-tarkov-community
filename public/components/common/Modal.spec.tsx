import React, { useState } from "react"
import { fireEvent, render, screen } from "@testing-library/react"
import { Modal } from "./Modal"

beforeEach(() => {
  const root = document.createElement("div")
  root.id = "root-modal"
  document.body.appendChild(root)
})

afterEach(() => document.getElementById("root-modal")?.remove())

test("a labelled modal owns keyboard focus and restores its trigger on Escape", () => {
  const Example = () => {
    const [open, setOpen] = useState(false)
    return <>
      <button onClick={() => setOpen(true)}>Open dialog</button>
      <Modal.Window isOpen={open} labelledBy="dialog-title" manageHistory={false} onClose={() => setOpen(false)}>
        <h2 id="dialog-title">Edit image</h2>
        <input aria-label="Image name" />
        <button onClick={() => setOpen(false)}>Close dialog</button>
      </Modal.Window>
    </>
  }
  render(<Example />)
  const trigger = screen.getByRole("button", { name: "Open dialog" })
  trigger.focus()
  fireEvent.click(trigger)
  expect(screen.getByRole("dialog", { name: "Edit image" })).toBeInTheDocument()
  expect(screen.getByLabelText("Image name")).toHaveFocus()

  const close = screen.getByRole("button", { name: "Close dialog" })
  close.focus()
  fireEvent.keyDown(document, { key: "Tab" })
  expect(screen.getByLabelText("Image name")).toHaveFocus()
  fireEvent.keyDown(document, { key: "Tab", shiftKey: true })
  expect(close).toHaveFocus()
  fireEvent.keyDown(document, { key: "Escape" })
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  expect(trigger).toHaveFocus()
})

test("an autofocus child preserves the trigger for focus restoration", () => {
  const Example = () => {
    const [open, setOpen] = useState(false)
    return <>
      <button onClick={() => setOpen(true)}>Sign in</button>
      <Modal.Window isOpen={open} manageHistory={false} onClose={() => setOpen(false)}>
        <input aria-label="Email" autoFocus />
        <button onClick={() => setOpen(false)}>Cancel</button>
      </Modal.Window>
    </>
  }
  render(<Example />)
  const trigger = screen.getByRole("button", { name: "Sign in" })
  trigger.focus()
  fireEvent.click(trigger)
  expect(screen.getByLabelText("Email")).toHaveFocus()

  fireEvent.keyDown(document, { key: "Escape" })
  expect(trigger).toHaveFocus()
})
