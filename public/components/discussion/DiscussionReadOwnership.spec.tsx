import React from "react"
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { DiscussionComment, DiscussionPage } from "@fider/models"
import { loadCommentContext, loadCommentRecords, loadComments } from "@fider/services/discussion"
import { prepareReadingPosition } from "@fider/services/readingPosition"
import { Discussion } from "./Discussion"

jest.mock("@fider/hooks", () => ({ useFider: () => ({ session: { isAuthenticated: true } }) }))
jest.mock("@fider/components", () => ({
  Button: jest.requireActual("@fider/components/common/Button").Button,
  SignInModal: () => null,
}))
jest.mock("./CommentComposer", () => ({ CommentComposer: () => <form aria-label="New comment" /> }))
jest.mock("./DiscussionCommentCard", () => ({
  DiscussionCommentCard: ({ comment, onChanged }: {
    comment: DiscussionComment
    onChanged: (comment: DiscussionComment, change: "edit" | "delete") => void
  }) => (
    <article id={`comment-${comment.id}`} data-state={comment.state}>
      <span>{comment.content}</span>
      <button onClick={() => onChanged({ ...comment, content: "Local edit", editedAt: "2026-09-27T00:00:03Z" }, "edit")}>
        Confirm edit {comment.id}
      </button>
      <button onClick={() => onChanged({ ...comment, state: "deleted" }, "delete")}>
        Confirm delete {comment.id}
      </button>
    </article>
  ),
}))
jest.mock("@fider/services/discussion", () => ({
  ...jest.requireActual("@fider/services/discussion"),
  loadComments: jest.fn(),
  loadCommentRecords: jest.fn(),
  loadCommentContext: jest.fn(),
}))

const ownerPermissions = { comment: false, signInToComment: false, react: false, images: false }

const owner = { kind: "post" as const, id: 1, number: 1, title: "Post", url: "/posts/1" }
const parent: DiscussionComment = {
  id: 1,
  parentId: null,
  hasReplies: true,
  content: "Parent comment",
  createdAt: "2026-09-27T00:00:00Z",
  user: null,
  state: "visible",
  permissions: { edit: false, delete: false, moderate: false, react: false, reply: false, report: false },
}
const page: DiscussionPage = { owner, comments: [parent], permissions: { comment: true, signInToComment: false, react: false, images: false } }

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((complete) => { resolve = complete })
  return { promise, resolve }
}

beforeEach(() => {
  jest.resetAllMocks()
  prepareReadingPosition(undefined)
  window.history.replaceState(null, "", owner.url)
  window.scrollBy = jest.fn()
  Element.prototype.scrollIntoView = jest.fn()
  window.IntersectionObserver = jest.fn(() => ({ observe: jest.fn(), disconnect: jest.fn() })) as any
  window.ResizeObserver = jest.fn(() => ({ observe: jest.fn(), disconnect: jest.fn() })) as any
  jest.mocked(loadComments).mockResolvedValue({ ok: true, data: page })
})

test("collapsing and reopening a thread preserves its pending reply read", async () => {
  const branch = deferred<Awaited<ReturnType<typeof loadComments>>>()
  jest.mocked(loadComments).mockImplementation((_owner, _sort, parentId) =>
    parentId === 1 ? branch.promise : Promise.resolve({ ok: true, data: page })
  )

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByText("Parent comment")
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  expect(screen.getByRole("button", { name: "Load more comments" })).toBeDisabled()

  fireEvent.click(screen.getByRole("button", { name: "Collapse thread for comment 1" }))
  fireEvent.click(screen.getByRole("button", { name: "Expand thread for comment 1" }))
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await act(async () => {})

  expect(screen.getByRole("button", { name: "Load more comments" })).toBeDisabled()
  expect(loadComments).toHaveBeenCalledTimes(2)
  await act(async () => branch.resolve({ ok: true, data: { ...page, comments: [] } }))
  await waitFor(() => expect(screen.queryByRole("button", { name: "Load more comments" })).toBeNull())
})

test("deleting a parent immediately promotes its replies while refreshing the branch", async () => {
  const reply = { ...parent, id: 2, parentId: 1, hasReplies: false, content: "Surviving reply" }
  jest.mocked(loadComments).mockImplementation(async (_owner, _sort, parentId) => ({
    ok: true,
    data: { ...page, comments: parentId === 1 ? [reply] : [parent] },
  }))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByText("Parent comment")
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByText("Surviving reply")

  const refreshed = deferred<Awaited<ReturnType<typeof loadComments>>>()
  jest.mocked(loadComments).mockReturnValue(refreshed.promise)
  fireEvent.click(screen.getByRole("button", { name: "Confirm delete 1" }))

  expect(document.getElementById("comment-1")).toBeNull()
  expect(screen.getByText("Surviving reply")).toBeVisible()

  await act(async () => refreshed.resolve({
    ok: true,
    data: { ...page, comments: [{ ...reply, parentId: null }] },
  }))
  expect(document.getElementById("comment-1")).toBeNull()
  expect(screen.getAllByText("Surviving reply")).toHaveLength(1)
})

test("changing sort supersedes an error from restoration hydration", async () => {
  prepareReadingPosition({
    "discussion:post:1": {
      sort: "liked",
      comments: [{ id: 1, parentId: null, hasReplies: false, collapsed: false, pending: "unloaded" }],
      branches: { 0: { ids: [1] } },
      collapsed: {},
      expanded: {},
      measurements: [],
    },
  })
  jest.mocked(loadCommentRecords).mockResolvedValue({ ok: false, status: 503, error: { errors: [{ message: "Old hydration failure" }] } })
  jest.mocked(loadComments).mockResolvedValue({
    ok: true,
    data: { ...page, comments: [{ ...parent, id: 2, hasReplies: false, content: "Current sorted comment" }] },
  })

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText("Old hydration failure")
  fireEvent.change(screen.getByRole("combobox", { name: "Sort discussion" }), { target: { value: "latest" } })

  await screen.findByText("Current sorted comment")
  expect(screen.queryByText("Old hydration failure")).toBeNull()
})

test("an older branch read cannot restore capabilities after a newer restricted snapshot", async () => {
  const first = deferred<Awaited<ReturnType<typeof loadComments>>>()
  const second = deferred<Awaited<ReturnType<typeof loadComments>>>()
  jest.mocked(loadComments).mockImplementation((_owner, _sort, parentId) => {
    if (parentId === 1) return first.promise
    if (parentId === 2) return second.promise
    return Promise.resolve({ ok: true, data: { ...page, comments: [parent, { ...parent, id: 2, content: "Second parent" }] } })
  })

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByText("Second parent")
  const loaders = screen.getAllByRole("button", { name: "Load more comments" })
  fireEvent.click(loaders[0])
  fireEvent.click(loaders[1])

  await act(async () => second.resolve({
    ok: true,
    data: {
      ...page,
      comments: [{ ...parent, id: 20, parentId: 2, hasReplies: false, content: "Second reply" }],
      permissions: { ...page.permissions, comment: false },
    },
  }))
  await waitFor(() => expect(screen.queryByRole("form", { name: "New comment" })).toBeNull())
  await act(async () => first.resolve({
    ok: true,
    data: { ...page, comments: [{ ...parent, id: 10, parentId: 1, hasReplies: false, content: "First reply" }] },
  }))

  await screen.findByText("First reply")
  expect(screen.getByText("Second reply")).toBeVisible()
  expect(screen.queryByRole("form", { name: "New comment" })).toBeNull()
})

test.each([false, true])("overlapping context reads distinguish local mutation=%s", async (editLocally) => {
  const list = deferred<Awaited<ReturnType<typeof loadComments>>>()
  const context = deferred<Awaited<ReturnType<typeof loadCommentContext>>>()
  jest.mocked(loadComments).mockReturnValue(list.promise)
  jest.mocked(loadCommentContext).mockReturnValue(context.promise)

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  window.history.replaceState(null, "", `${owner.url}#comment-1`)
  fireEvent(window, new HashChangeEvent("hashchange"))

  await act(async () => list.resolve({
    ok: true,
    data: { ...page, comments: [{ ...parent, hasReplies: false, content: "Older read", editedAt: "2026-09-27T00:00:01Z" }] },
  }))
  await screen.findByText("Older read")
  if (editLocally) {
    fireEvent.click(screen.getByRole("button", { name: "Confirm edit 1" }))
  }
  await act(async () => context.resolve({
    ok: true,
    data: {
      ...page,
      commentId: 1,
      comments: [{ ...parent, hasReplies: false, content: "Newer read", editedAt: "2026-09-27T00:00:02Z" }],
    },
  }))

  expect(screen.getByText(editLocally ? "Local edit" : "Newer read")).toBeVisible()
  expect(screen.queryByText("Older read")).toBeNull()
})

test.each(["visible", "deleted"] as const)("a later-started context read preserves newer server state=%s", async (state) => {
  const list = deferred<Awaited<ReturnType<typeof loadComments>>>()
  const context = deferred<Awaited<ReturnType<typeof loadCommentContext>>>()
  jest.mocked(loadComments).mockReturnValue(list.promise)
  jest.mocked(loadCommentContext).mockReturnValue(context.promise)

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  window.history.replaceState(null, "", `${owner.url}#comment-1`)
  fireEvent(window, new HashChangeEvent("hashchange"))

  await act(async () => list.resolve({
    ok: true,
    data: {
      ...page,
      comments: [{
        ...parent,
        state,
        content: state === "deleted" ? "" : "Newer server state",
        editedAt: state === "deleted" ? undefined : "2026-09-27T00:00:02Z",
      }],
    },
  }))
  await waitFor(() => {
    if (state === "deleted") {
      expect(document.getElementById("comment-1")).toBeNull()
    } else {
      expect(document.getElementById("comment-1")).toHaveAttribute("data-state", state)
    }
  })
  await act(async () => context.resolve({
    ok: true,
    data: {
      ...page,
      commentId: 1,
      comments: [{ ...parent, content: "Older server snapshot", editedAt: "2026-09-27T00:00:01Z" }],
    },
  }))

  if (state === "deleted") {
    expect(document.getElementById("comment-1")).toBeNull()
  } else {
    expect(document.getElementById("comment-1")).toHaveAttribute("data-state", state)
  }
  if (state === "visible") {
    expect(screen.getByText("Newer server state")).toBeVisible()
  }
  expect(screen.queryByText("Older server snapshot")).toBeNull()
  expect(screen.getByRole("link", { name: "View full discussion" })).toBeVisible()
})

test("sort after moving away from a restored viewport renders the new branch loader", async () => {
  const comments = Array.from({ length: 100 }, (_, index) => ({
    ...parent,
    id: index + 1,
    hasReplies: false,
    content: `Restored comment ${index + 1}`,
  }))
  prepareReadingPosition({
    "discussion:post:1": {
      sort: "liked",
      comments: comments.map(({ id, parentId, hasReplies }) => ({ id, parentId, hasReplies, collapsed: false, pending: "unloaded" })),
      branches: { 0: { ids: comments.map(({ id }) => id) } },
      collapsed: {},
      expanded: {},
      measurements: [],
      viewport: { top: 16000, bottom: 16800 },
    },
  })
  jest.mocked(loadCommentRecords).mockImplementation((_owner, ids) => Promise.resolve({
    ok: true,
    data: { ...page, comments: comments.filter(({ id }) => ids.includes(id)) },
  }))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText("Restored comment 67")

  await act(async () => fireEvent.scroll(window))
  await screen.findByText("Restored comment 1")

  await act(async () => {
    fireEvent.change(screen.getByRole("combobox", { name: "Sort discussion" }), { target: { value: "latest" } })
  })

  expect(screen.getByRole("button", { name: "Load more comments" })).toBeVisible()
})

test.each(["visible", "deleted"] as const)("an earlier-started read can carry newer server state=%s", async (state) => {
  const list = deferred<Awaited<ReturnType<typeof loadComments>>>()
  const context = deferred<Awaited<ReturnType<typeof loadCommentContext>>>()
  jest.mocked(loadComments).mockReturnValue(list.promise)
  jest.mocked(loadCommentContext).mockReturnValue(context.promise)

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  window.history.replaceState(null, "", `${owner.url}#comment-1`)
  fireEvent(window, new HashChangeEvent("hashchange"))

  await act(async () => context.resolve({
    ok: true,
    data: {
      ...page,
      commentId: 1,
      comments: [{ ...parent, content: "Older snapshot", editedAt: "2026-09-27T00:00:01Z" }],
    },
  }))
  await screen.findByText("Older snapshot")
  await act(async () => list.resolve({
    ok: true,
    data: {
      ...page,
      comments: [{
        ...parent,
        state,
        content: state === "visible" ? "Newer edit" : "",
        editedAt: state === "visible" ? "2026-09-27T00:00:02Z" : undefined,
      }],
    },
  }))

  if (state === "deleted") {
    expect(document.getElementById("comment-1")).toBeNull()
  } else {
    expect(document.getElementById("comment-1")).toHaveAttribute("data-state", state)
  }
  expect(screen.queryByText("Older snapshot")).toBeNull()
  if (state === "visible") {
    expect(screen.getByText("Newer edit")).toBeVisible()
  }
})
