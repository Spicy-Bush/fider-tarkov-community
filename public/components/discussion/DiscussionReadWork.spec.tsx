import React from "react"
import { act, fireEvent, render, screen } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { DiscussionComment } from "@fider/models"
import { loadComments } from "@fider/services/discussion"
import { prepareReadingPosition } from "@fider/services/readingPosition"
import { Discussion } from "./Discussion"
import { DiscussionRow } from "./DiscussionViewport"

const mockRows = jest.fn()

jest.mock("@fider/hooks", () => ({ useFider: () => ({ session: { isAuthenticated: true } }) }))
jest.mock("@fider/components", () => ({
  Button: jest.requireActual("@fider/components/common/Button").Button,
  SignInModal: () => null,
}))
jest.mock("./CommentComposer", () => ({ CommentComposer: () => null }))
jest.mock("./DiscussionViewport", () => ({
  DiscussionViewport: ({ rows, children }: { rows: DiscussionRow[]; children: (row: DiscussionRow) => React.ReactNode }) => {
    mockRows(rows)
    return <>{rows.filter((row) => row.kind === "load").map((row, index) => (
      <div key={index}>{children(row)}</div>
    ))}</>
  },
}))
jest.mock("@fider/services/discussion", () => ({
  ...jest.requireActual("@fider/services/discussion"),
  loadComments: jest.fn(),
}))

const owner = { kind: "post" as const, id: 1, number: 1, title: "Post", url: "/posts/1" }
const ownerPermissions = { comment: false, signInToComment: false, react: false, images: false }
const comment: DiscussionComment = {
  id: 1,
  parentId: null,
  hasReplies: false,
  content: "Comment",
  createdAt: "2026-09-27T00:00:00Z",
  user: null,
  state: "visible",
  permissions: { edit: false, delete: false, moderate: false, react: false, reply: false, report: false },
}

beforeEach(() => {
  jest.resetAllMocks()
  prepareReadingPosition(undefined)
  window.history.replaceState(null, "", owner.url)
  window.IntersectionObserver = jest.fn(() => ({ observe: jest.fn(), disconnect: jest.fn() })) as any
})

test.each([3000, 50000])("pending and failed reads preserve rows for %i retained comments", async (count) => {
  let complete!: (value: Awaited<ReturnType<typeof loadComments>>) => void
  const pending = new Promise<Awaited<ReturnType<typeof loadComments>>>((resolve) => {
    complete = resolve
  })
  jest.mocked(loadComments)
    .mockResolvedValueOnce({
      ok: true,
      data: {
        owner,
        comments: Array.from({ length: count }, (_, index) => ({ ...comment, id: index + 1 })),
        permissions: { comment: true, signInToComment: false, react: false, images: false },
        next: "continuation",
      },
    })
    .mockReturnValueOnce(pending)

  render(<Discussion owner={owner} ownerPermissions={ownerPermissions} />)
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Load more comments" })))
  const rows = mockRows.mock.calls[mockRows.mock.calls.length - 1][0] as DiscussionRow[]
  expect(rows).toHaveLength(count + 1)
  mockRows.mockClear()

  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  expect(screen.getByRole("button", { name: "Load more comments" })).toBeDisabled()
  await act(async () => complete({ ok: false, status: 503, error: {} }))
  expect(screen.getByRole("button", { name: "Retry loading comments" })).toBeEnabled()

  expect(mockRows).toHaveBeenCalled()
  for (const [rendered] of mockRows.mock.calls) {
    expect(rendered).toBe(rows)
  }
})
