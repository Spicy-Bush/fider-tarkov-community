import React, { useState } from "react"
import { createPortal } from "react-dom"
import { act, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, beforeEach, expect, jest, test } from "@jest/globals"
import { DiscussionRow, DiscussionViewport } from "./DiscussionViewport"

const rows: DiscussionRow[] = Array.from({ length: 3000 }, (_, index) => ({
  kind: "comment",
  id: index + 1,
  depth: 0,
  end: index + 1,
  collapsed: false,
}))

let scrollTop = 0

beforeEach(() => {
  scrollTop = 0
  jest.useFakeTimers()
  window.ResizeObserver = jest.fn(() => ({ observe: jest.fn(), disconnect: jest.fn() })) as any
  HTMLElement.prototype.scrollIntoView = jest.fn()

  jest.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    const top = this.classList.contains("discussion-threads") ? -scrollTop : 0

    return {
      top,
      bottom: top + 720000,
      left: 0,
      right: 1000,
      width: 1000,
      height: 720000,
      x: 0,
      y: top,
      toJSON: () => ({}),
    }
  })
})

afterEach(() => {
  jest.restoreAllMocks()
  jest.useRealTimers()
})

function Card({ id }: { id: number }) {
  const [open, setOpen] = useState(false)

  return (
    <article id={`comment-${id}`}>
      <button onClick={() => setOpen(true)}>Open comment {id}</button>
      {open && createPortal(<button onClick={() => setOpen(false)}>Close dialog</button>, document.body)}
    </article>
  )
}

function discussion(target?: number) {
  return render(
    <DiscussionViewport rows={rows} target={target} onCollapse={() => {}}>
      {(row) => row.kind === "comment" && <Card id={row.id} />}
    </DiscussionViewport>
  )
}

function scrollToMiddle() {
  act(() => {
    scrollTop = 360000
    window.dispatchEvent(new Event("scroll"))
    jest.advanceTimersByTime(20)
  })
}

test("loading thousands of comments retains only nearby cards", () => {
  discussion()
  expect(screen.getAllByRole("article").length).toBeLessThan(30)
  expect(screen.getByRole("button", { name: "Open comment 1" })).toBeVisible()

  scrollToMiddle()

  expect(screen.getAllByRole("article").length).toBeLessThan(30)
  expect(screen.queryByRole("button", { name: "Open comment 1" })).toBeNull()
  expect(screen.getByRole("button", { name: "Open comment 1501" })).toBeVisible()
})

test("focus in a portal retains its comment when the background scrolls", () => {
  discussion()
  fireEvent.click(screen.getByRole("button", { name: "Open comment 1" }))
  fireEvent.focus(screen.getByRole("button", { name: "Close dialog" }))

  scrollToMiddle()

  expect(screen.getByRole("button", { name: "Close dialog" })).toBeVisible()
  expect(screen.getByRole("button", { name: "Open comment 1" })).toBeVisible()
  expect(screen.getAllByRole("article").length).toBeLessThan(30)
})

test("a linked comment mounts before scrolling even outside the visible range", () => {
  const scrolled: { id: string; mounted: boolean }[] = []
  HTMLElement.prototype.scrollIntoView = jest.fn(function (this: HTMLElement) {
    scrolled.push({ id: this.id, mounted: this.isConnected })
  })

  discussion(2999)

  expect(scrolled).toEqual([{ id: "comment-2999", mounted: true }])
  expect(HTMLElement.prototype.scrollIntoView).toHaveBeenCalledWith({ block: "center" })
  expect(screen.getAllByRole("article").length).toBeLessThan(30)
})
