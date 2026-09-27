import React from "react"
import { render } from "@testing-library/react"
import { expect, test } from "@jest/globals"
import { useBodyScrollLock } from "./useBodyScrollLock"

function Overlay({ open }: { open: boolean }) {
  useBodyScrollLock(open)
  return null
}

test("closed or removed overlays cannot unlock another open overlay", () => {
  document.body.style.overflow = "auto"

  const first = render(<Overlay open />)
  const closed = render(<Overlay open={false} />)
  const second = render(<Overlay open />)
  expect(document.body.style.overflow).toBe("hidden")

  closed.unmount()
  first.rerender(<Overlay open={false} />)
  expect(document.body.style.overflow).toBe("hidden")

  second.unmount()
  expect(document.body.style.overflow).toBe("auto")

  first.unmount()
  document.body.style.overflow = ""
})
