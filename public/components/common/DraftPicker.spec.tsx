import React from "react"
import { act, fireEvent, render, screen } from "@testing-library/react"
import { AccountDraft } from "@fider/services/browserDrafts"
import { DraftPicker } from "./DraftPicker"

const pending: AccountDraft<{ content: string; attachments: []; parentId: number }> = {
  id: "sealed-comment",
  kind: "comment",
  phase: "pending",
  scope: "1:post:7:13",
  revision: 4,
  payload: { content: "Prepared reply", attachments: [], parentId: 13 },
  updatedAt: Date.now(),
}

test("selection preserves every draft and follows replacement of the list", async () => {
  const drafts = Array.from({ length: 120 }, (_, index) => ({
    ...pending,
    id: `pending-${index}`,
    payload: { ...pending.payload, content: `Draft ${index}` },
  }))
  const onResume = jest.fn().mockResolvedValue(undefined)
  const view = render(<DraftPicker drafts={drafts} label="Pending submissions" action="Continue pending submission" onSelect={onResume} />)

  expect(screen.getAllByRole("button")).toHaveLength(1)
  expect(screen.getAllByRole("option")).toHaveLength(120)
  fireEvent.change(screen.getByRole("combobox", { name: "Pending submissions" }), { target: { value: "pending-119" } })
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Continue pending submission" }))
  })
  expect(onResume).toHaveBeenCalledWith(drafts[119])

  view.rerender(<DraftPicker drafts={drafts.slice(0, 2)} label="Pending submissions" action="Continue pending submission" onSelect={onResume} />)
  expect(screen.getByRole("combobox")).toHaveValue("pending-0")
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Continue pending submission" }))
  })
  expect(onResume).toHaveBeenLastCalledWith(drafts[0])

  view.rerender(<DraftPicker drafts={[]} label="Pending submissions" action="Continue pending submission" onSelect={onResume} />)
  expect(screen.queryByRole("combobox")).toBeNull()
})
