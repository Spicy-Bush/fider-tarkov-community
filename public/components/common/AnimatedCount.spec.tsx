import React, { Suspense, startTransition } from "react"
import { act, render, screen } from "@testing-library/react"
import { AnimatedCount } from "./AnimatedCount"

test("animation direction follows the last displayed count after an interrupted render", () => {
  const pending = new Promise<void>(() => {})

  function Wait({ active }: { active: boolean }) {
    if (active) {
      throw pending
    }

    return null
  }

  function Counter({ value, pending }: { value: number; pending: boolean }) {
    return (
      <Suspense fallback={<span>Loading</span>}>
        <AnimatedCount value={value} />
        <Wait active={pending} />
      </Suspense>
    )
  }

  const view = render(<Counter value={0} pending={false} />)
  expect(screen.getByText("0")).not.toHaveClass("count-roll-up", "count-roll-down")

  act(() => {
    startTransition(() => view.rerender(<Counter value={2} pending />))
  })
  expect(screen.getByText("0")).toBeVisible()

  view.rerender(<Counter value={1} pending={false} />)
  expect(screen.getByText("1")).toHaveClass("count-roll-up")

  view.rerender(<Counter value={-1} pending={false} />)
  expect(screen.getByText("-1")).toHaveClass("count-roll-down")
})
