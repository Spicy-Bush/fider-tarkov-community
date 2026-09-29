import React from "react"
import { act, fireEvent, render, screen } from "@testing-library/react"
import { DraftStatus } from "./DraftStatus"

test("routine draft saves do not add layout, while failures remain recoverable", async () => {
  const flush = jest.fn().mockResolvedValue({})
  const props = { alternatives: [], resume: jest.fn(), flush }
  const view = render(<DraftStatus {...props} status="loading" />)

  for (const status of ["idle", "loading", "saving", "saved"] as const) {
    view.rerender(<DraftStatus {...props} status={status} />)
    expect(view.container).toBeEmptyDOMElement()
  }

  view.rerender(<DraftStatus {...props} status="error" error="Browser storage is full." />)
  expect(screen.getByText("Browser storage is full.")).toBeVisible()
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Retry draft save" }))
  })
  expect(flush).toHaveBeenCalledTimes(1)

  const saved = {
    id: "saved",
    revision: 1,
    kind: "post" as const,
    phase: "editable" as const,
    scope: "post:new",
    payload: { title: "Saved suggestion", attachments: [] },
    updatedAt: 100,
  }
  props.resume.mockResolvedValue(undefined)
  view.rerender(<DraftStatus {...props} alternatives={[saved]} status="saving" />)
  expect(screen.getByRole("button", { name: "Restore draft" })).toBeDisabled()

  view.rerender(<DraftStatus {...props} alternatives={[saved]} status="saved" />)
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Restore draft" }))
  })
  expect(props.resume).toHaveBeenCalledWith(saved)
})
