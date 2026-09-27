import React from "react"
import { fireEvent, render, screen } from "@testing-library/react"
import { expect, test } from "@jest/globals"
import { Reactions } from "./Reactions"

jest.mock("@fider/hooks", () => ({
  useFider: () => ({ session: { isAuthenticated: true } }),
}))

test("pending reactions preserve controls, emoji nodes, counts and focus", () => {
  const props = {
    emojiSelectorRef: React.createRef<HTMLDivElement>(),
    reactions: [{ emoji: "👍", count: 2, includesMe: false }],
    toggleReaction: jest.fn(),
  }
  const view = render(<Reactions {...props} />)
  const picker = screen.getByRole("button", { name: "Add reaction" })
  const reaction = screen.getByRole("button", { name: "👍 2" })
  const emoji = reaction.firstElementChild

  reaction.focus()
  fireEvent.click(reaction)
  view.rerender(<Reactions {...props} busy />)

  expect(screen.getByRole("button", { name: "Add reaction" })).toBe(picker)
  expect(screen.getByRole("button", { name: "👍 2" })).toBe(reaction)
  expect(reaction.firstElementChild).toBe(emoji)
  expect(reaction).toHaveAttribute("aria-disabled", "true")
  expect(reaction).toHaveFocus()

  fireEvent.click(reaction)
  fireEvent.click(picker)

  expect(props.toggleReaction).toHaveBeenCalledTimes(1)
  expect(picker).toHaveAttribute("aria-expanded", "false")

  view.rerender(<Reactions {...props} reactions={[{ emoji: "👍", count: 3, includesMe: true }]} />)

  expect(screen.getByRole("button", { name: "Add reaction" })).toBe(picker)
  expect(screen.getByRole("button", { name: "👍 3" })).toBe(reaction)
  expect(reaction.firstElementChild).toBe(emoji)
  expect(reaction).toHaveAttribute("aria-pressed", "true")
  expect(reaction).toHaveFocus()
})

test("selecting an emoji returns focus to the persistent picker button", () => {
  const toggleReaction = jest.fn()
  render(<Reactions emojiSelectorRef={React.createRef<HTMLDivElement>()} toggleReaction={toggleReaction} />)

  const picker = screen.getByRole("button", { name: "Add reaction" })
  fireEvent.click(picker)
  fireEvent.click(screen.getByRole("button", { name: "😂" }))

  expect(toggleReaction).toHaveBeenCalledWith("😂")
  expect(picker).toHaveFocus()
  expect(picker).toHaveAttribute("aria-expanded", "false")
})
