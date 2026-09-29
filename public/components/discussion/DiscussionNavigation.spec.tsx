import React from "react"
import { act, fireEvent, render, screen } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { Discussion } from "./Discussion"
import { loadCommentContext, loadComments } from "@fider/services/discussion"
import { CommentContext, DiscussionComment } from "@fider/models"
import { Result } from "@fider/services"

let mockEditedAt: string | undefined

jest.mock("@fider/hooks", () => ({
  useFider: () => ({ session: { isAuthenticated: true } }),
}))

jest.mock("@fider/components", () => ({
  Button: jest.requireActual("@fider/components/common/Button").Button,
  SignInModal: () => null,
}))

jest.mock("./CommentComposer", () => ({ CommentComposer: () => null }))
jest.mock("./DiscussionCommentCard", () => ({
  DiscussionCommentCard: ({ comment, onCreated, onChanged, onReactionsChanged }: {
    comment: DiscussionComment
    onCreated: (reply: DiscussionComment) => void
    onChanged: (reply: DiscussionComment, change: "edit" | "moderation" | "report") => void
    onReactionsChanged: (id: number, reactions: { emoji: string; count: number; includesMe: boolean }[]) => void
  }) => (
    <article id={`comment-${comment.id}`}>
      {comment.content}
      <span>Reactions: {comment.reactionCounts?.[0]?.count || 0}</span>
      <button onClick={() => onCreated({ ...comment, id: comment.id + 100, parentId: comment.id, content: "New reply", hasReplies: false })}>
        Save reply to {comment.id}
      </button>
      <button onClick={() => onChanged({ ...comment, content: "Confirmed edit", editedAt: mockEditedAt }, "edit")}>
        Edit comment {comment.id}
      </button>
      <button onClick={() => onReactionsChanged(comment.id, [{ emoji: "👍", count: 1, includesMe: true }])}>
        React to {comment.id}
      </button>
      <button onClick={() => onCreated({ ...comment, content: `Comment ${comment.id}`, reactionCounts: [] })}>
        Recover creation of {comment.id}
      </button>
      <button disabled={!comment.permissions.react}>Reaction control</button>
      {comment.permissions.report && <span>Reporting available</span>}
      <button onClick={() => onChanged(comment, "report")}>Confirm report</button>
      <button onClick={() => onChanged({
        ...comment,
        moderationPending: false,
        permissions: { ...comment.permissions, react: true, report: true },
      }, "moderation")}>Confirm unhide</button>
    </article>
  ),
}))

jest.mock("@fider/services/discussion", () => ({
  ...jest.requireActual("@fider/services/discussion"),
  loadCommentContext: jest.fn(),
  loadComments: jest.fn(),
}))

const ownerPermissions = { comment: false, signInToComment: false, react: false, images: false }

const owner = { kind: "page" as const, id: 1, title: "Page", url: "/pages/page" }

function response(id: number): Result<CommentContext> {
  return {
    ok: true,
    data: {
      owner,
      commentId: id,
      permissions: { comment: true, signInToComment: false, react: true, images: false },
      comments: [{
        id,
        parentId: null,
        content: `Comment ${id}`,
        createdAt: "2026-09-26T00:00:00Z",
        hasReplies: false,
        state: "visible",
        permissions: { edit: true, delete: true, moderate: false, reply: true, react: true, report: false },
      }],
    },
  }
}

beforeEach(() => {
  jest.clearAllMocks()
  mockEditedAt = undefined
  window.history.replaceState(null, "", "/pages/page#comment-1")
  HTMLElement.prototype.scrollIntoView = jest.fn()
  window.IntersectionObserver = jest.fn(() => ({ observe: jest.fn(), disconnect: jest.fn() })) as any
  window.ResizeObserver = jest.fn(() => ({ observe: jest.fn(), disconnect: jest.fn() })) as any
})

test.each(["success", "failure", "exception"])("late context %s cannot replace a newer thread", async (outcome) => {
  let complete!: (result: Result<CommentContext>) => void
  let fail!: (error: Error) => void
  const first = new Promise<Result<CommentContext>>((resolve, reject) => {
    complete = resolve
    fail = reject
  })
  jest.mocked(loadCommentContext).mockReturnValueOnce(first).mockResolvedValueOnce(response(2))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)

  await act(async () => {
    window.history.replaceState(null, "", "/pages/page#comment-2")
    window.dispatchEvent(new HashChangeEvent("hashchange"))
  })

  expect(screen.getByText("Comment 2")).toBeVisible()

  await act(async () => {
    if (outcome === "exception") {
      fail(new Error("Connection lost"))
    } else {
      complete(outcome === "success" ? response(1) : { ok: false, status: 404, error: {} })
    }
  })

  expect(screen.getByText("Comment 2")).toBeVisible()
  expect(screen.queryByText("Comment 1")).toBeNull()
  expect(screen.queryByRole("alert")).toBeNull()
})

test("returning to the full discussion invalidates a pending thread response", async () => {
  let complete!: (result: Result<CommentContext>) => void
  jest.mocked(loadCommentContext).mockReturnValueOnce(new Promise((resolve) => {
    complete = resolve
  }))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)

  await act(async () => {
    window.history.replaceState(null, "", "/pages/page")
    window.dispatchEvent(new HashChangeEvent("hashchange"))
    complete(response(1))
  })

  expect(screen.queryByText("Comment 1")).toBeNull()
  expect(screen.queryByText("View full discussion")).toBeNull()
  expect(screen.queryByRole("alert")).toBeNull()
})

test("a stalled old sort cannot block the newly selected order", async () => {
  window.history.replaceState(null, "", "/pages/page")
  let complete!: (result: Result<CommentContext>) => void
  jest.mocked(loadComments).mockReturnValueOnce(new Promise((resolve) => {
    complete = resolve
  })).mockResolvedValueOnce(response(2))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  expect(loadComments).toHaveBeenCalledWith(owner, "liked", undefined, undefined, 5, expect.any(AbortSignal))

  fireEvent.change(screen.getByRole("combobox", { name: "Sort discussion" }), { target: { value: "latest" } })
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await act(async () => {})

  expect(loadComments).toHaveBeenCalledWith(owner, "latest", undefined, undefined, 5, expect.any(AbortSignal))
  expect(screen.getByText("Comment 2")).toBeVisible()

  await act(async () => complete(response(1)))

  expect(screen.getByText("Comment 2")).toBeVisible()
  expect(screen.queryByText("Comment 1")).toBeNull()
})

test("changing sort preserves the linked comment after clearing the previous order", async () => {
  jest.mocked(loadCommentContext).mockResolvedValue(response(1))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText("Comment 1")

  fireEvent.change(screen.getByRole("combobox", { name: "Sort discussion" }), { target: { value: "replies" } })

  await screen.findByText("Comment 1")
  expect(loadCommentContext).toHaveBeenCalledTimes(2)
  expect(screen.getByText("View full discussion")).toBeVisible()
})

test("a reaction result preserves confirmed edit content", async () => {
  jest.mocked(loadCommentContext).mockResolvedValue(response(1))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText("Comment 1")

  fireEvent.click(screen.getByRole("button", { name: "Edit comment 1" }))
  fireEvent.click(screen.getByRole("button", { name: "React to 1" }))

  expect(screen.getByText("Confirmed edit")).toBeVisible()
  expect(screen.getByText("Reactions: 1")).toBeVisible()
})

test("a repeated creation receipt preserves later edits and reactions", async () => {
  jest.mocked(loadCommentContext).mockResolvedValue(response(1))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText("Comment 1")

  fireEvent.click(screen.getByRole("button", { name: "Edit comment 1" }))
  fireEvent.click(screen.getByRole("button", { name: "React to 1" }))
  fireEvent.click(screen.getByRole("button", { name: "Recover creation of 1" }))

  expect(screen.getByText("Confirmed edit")).toBeVisible()
  expect(screen.getByText("Reactions: 1")).toBeVisible()
})

test.each<[string, string | undefined, boolean]>([
  ["2026-09-26T00:00:00.000010Z", "2026-09-26T00:00:00.000009Z", false],
  ["2026-09-26T00:00:00.100001Z", "2026-09-26T00:00:00.100Z", false],
  ["2026-09-26T02:00:00.100001+02:00", "2026-09-26T00:00:00.100Z", false],
  ["2026-09-26T00:00:01Z", "2026-09-26T00:00:00.999999Z", false],
  ["2026-09-26T00:00:01Z", undefined, false],
  ["2026-09-26T02:00:00+02:00", "2026-09-26T00:00:00.000Z", false],
  ["2026-09-26T00:00:00.000009Z", "2026-09-26T00:00:00.00001Z", true],
  ["2026-09-26T00:00:00.999999Z", "2026-09-26T00:00:01Z", true],
])("edit receipt ordering: stored %s, incoming %s, accept %s", async (current, incoming, accept) => {
  const initial = response(1)

  if (!initial.ok) {
    throw new Error("Expected successful fixture")
  }

  initial.data.comments[0].editedAt = current
  mockEditedAt = incoming
  jest.mocked(loadCommentContext).mockResolvedValue(initial)

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText("Comment 1")
  fireEvent.click(screen.getByRole("button", { name: "Edit comment 1" }))

  expect(screen.getByText(accept ? "Confirmed edit" : "Comment 1")).toBeVisible()
  expect(screen.queryByText(accept ? "Comment 1" : "Confirmed edit")).toBeNull()
})

test("unhiding restores reaction permission without undoing a confirmed report", async () => {
  const initial = response(1)

  if (!initial.ok) {
    throw new Error("Expected successful fixture")
  }

  initial.data.comments[0].permissions.react = false
  jest.mocked(loadCommentContext).mockResolvedValue(initial)

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText("Comment 1")
  expect(screen.getByRole("button", { name: "Reaction control" })).toBeDisabled()

  fireEvent.click(screen.getByRole("button", { name: "Confirm unhide" }))
  expect(screen.getByRole("button", { name: "Reaction control" })).toBeEnabled()
  expect(screen.getByText("Reporting available")).toBeVisible()

  fireEvent.click(screen.getByRole("button", { name: "Confirm report" }))
  fireEvent.click(screen.getByRole("button", { name: "Confirm unhide" }))
  expect(screen.queryByText("Reporting available")).toBeNull()
})

test("a prefetched chain retains a reply saved after the read began", async () => {
  const initial = response(1)
  const children = response(2)
  const descendant = response(3)
  if (!initial.ok || !children.ok || !descendant.ok) {
    throw new Error("Expected successful fixtures")
  }

  initial.data.comments[0].hasReplies = true
  children.data.comments[0].parentId = 1
  children.data.comments[0].hasReplies = true
  descendant.data.comments[0].parentId = 2
  initial.data.comments.push(children.data.comments[0])
  children.data.replies = descendant.data.comments

  let complete!: (result: Result<CommentContext>) => void
  jest.mocked(loadCommentContext).mockResolvedValueOnce(initial)
  jest.mocked(loadComments).mockReturnValueOnce(new Promise((resolve) => {
    complete = resolve
  }))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText("Comment 2")

  const loaders = screen.getAllByRole("button", { name: "Load more comments" })
  fireEvent.click(loaders[loaders.length - 1])
  expect(loadComments).toHaveBeenCalledWith(owner, "liked", 1, undefined, 4, expect.any(AbortSignal))

  fireEvent.click(screen.getByRole("button", { name: "Save reply to 2" }))
  expect(screen.getByText("New reply")).toBeVisible()

  await act(async () => complete(children))

  expect(screen.getByText("Comment 3")).toBeVisible()
  expect(screen.getByText("New reply")).toBeVisible()
})

test.each(["list", "context"])("a late %s response cannot replace a confirmed edit", async (source) => {
  jest.mocked(loadCommentContext).mockResolvedValueOnce(response(1))
  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText("Comment 1")

  let complete!: (result: Result<CommentContext>) => void
  const pending = new Promise<Result<CommentContext>>((resolve) => {
    complete = resolve
  })

  if (source === "context") {
    jest.mocked(loadCommentContext).mockReturnValueOnce(pending)
    fireEvent(window, new HashChangeEvent("hashchange"))
  } else {
    window.history.replaceState(null, "", "/pages/page")
    fireEvent(window, new HashChangeEvent("hashchange"))
    jest.mocked(loadComments).mockReturnValueOnce(pending)
    fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  }

  fireEvent.click(screen.getByRole("button", { name: "Edit comment 1" }))
  expect(screen.getByText("Confirmed edit")).toBeVisible()

  await act(async () => complete(response(1)))

  expect(screen.getByText("Confirmed edit")).toBeVisible()
  expect(screen.queryByText("Comment 1")).toBeNull()
})
