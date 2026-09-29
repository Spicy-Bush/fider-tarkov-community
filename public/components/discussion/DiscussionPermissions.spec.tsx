import React from "react"
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { DiscussionComment, DiscussionOwner } from "@fider/models"
import { loadCommentContext, loadCommentRecords, loadComments } from "@fider/services/discussion"
import { captureReadingPosition, prepareReadingPosition } from "@fider/services/readingPosition"
import { Discussion } from "./Discussion"

jest.mock("@fider/components", () => ({
  Button: jest.requireActual("@fider/components/common/Button").Button,
  SignInModal: () => null,
}))
jest.mock("./CommentComposer", () => ({ CommentComposer: () => <form aria-label="New comment" /> }))
jest.mock("./DiscussionCommentCard", () => ({
  DiscussionCommentCard: ({ comment }: { comment: DiscussionComment }) => <article id={`comment-${comment.id}`}>{comment.content}</article>,
}))
jest.mock("@fider/services/discussion", () => ({
  ...jest.requireActual("@fider/services/discussion"),
  loadComments: jest.fn(),
  loadCommentRecords: jest.fn(),
  loadCommentContext: jest.fn(),
}))

const owner = { kind: "post" as const, id: 1, number: 1, title: "Post", url: "/posts/1" }
const allowed = { comment: true, signInToComment: false, react: true, images: false }
const restricted = { comment: false, signInToComment: false, react: false, images: false }
const comment: DiscussionComment = {
  id: 1,
  parentId: null,
  hasReplies: false,
  content: "New comment content",
  createdAt: "2026-09-26T00:00:00Z",
  user: null,
  state: "visible",
  permissions: { edit: false, delete: false, moderate: false, react: false, reply: false, report: false },
}

beforeEach(() => {
  jest.resetAllMocks()
  prepareReadingPosition(undefined)
  window.history.replaceState(null, "", owner.url)
  window.scrollBy = jest.fn()
  Element.prototype.scrollIntoView = jest.fn()
  window.IntersectionObserver = jest.fn(() => ({ observe: jest.fn(), disconnect: jest.fn() })) as any
  window.ResizeObserver = jest.fn(() => ({ observe: jest.fn(), disconnect: jest.fn() })) as any
  jest.mocked(loadComments).mockResolvedValue({ ok: true, data: { owner, comments: [], permissions: restricted } })
})

test.each([allowed, restricted])("offscreen restoration uses owner permissions with comment=$comment without a metadata read", (permissions) => {
  prepareReadingPosition({
    "discussion:post:1": {
      sort: "liked",
      comments: [{
        id: 1,
        parentId: null,
        hasReplies: false,
        collapsed: false,
        pending: "unloaded",
      }],
      branches: { 0: { ids: [1] } },
      collapsed: {},
      expanded: {},
      measurements: [],
      viewport: { top: 8000, bottom: 9000 },
    },
  })
  render(<Discussion owner={owner} ownerPermissions={permissions} />)

  if (permissions.comment) {
    expect(screen.getByRole("form", { name: "New comment" })).toBeVisible()
  } else {
    expect(screen.queryByRole("form", { name: "New comment" })).toBeNull()
    expect(screen.getByRole("region", { name: "Discussion" })).toBeVisible()
  }
  expect(screen.queryAllByRole("article")).toHaveLength(0)
  expect(loadComments).not.toHaveBeenCalled()
  expect(loadCommentRecords).not.toHaveBeenCalled()
})

test("a fresh parent receipt updates permissions while ordinary rerenders retain accepted reads", async () => {
  const rendered = render(<Discussion owner={owner} ownerPermissions={allowed} />)
  expect(screen.getByRole("form", { name: "New comment" })).toBeVisible()

  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await waitFor(() => expect(screen.queryByRole("region", { name: "Discussion" })).toBeNull())

  rendered.rerender(<Discussion owner={{ ...owner }} ownerPermissions={allowed} />)
  expect(screen.queryByRole("form", { name: "New comment" })).toBeNull()

  rendered.rerender(<Discussion owner={{ ...owner }} ownerPermissions={{ ...allowed }} />)
  expect(screen.getByRole("form", { name: "New comment" })).toBeVisible()

  rendered.rerender(<Discussion owner={{ ...owner }} ownerPermissions={{ ...restricted }} />)
  expect(screen.queryByRole("form", { name: "New comment" })).toBeNull()
})

test("a parent receipt supersedes an unfinished child read but a later child read can update it", async () => {
  let complete!: (result: Awaited<ReturnType<typeof loadComments>>) => void
  jest.mocked(loadComments).mockReturnValueOnce(new Promise((resolve) => {
    complete = resolve
  }))

  const rendered = render(<Discussion owner={owner} ownerPermissions={allowed} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))

  const received = { ...restricted }
  rendered.rerender(<Discussion owner={owner} ownerPermissions={received} />)
  expect(screen.queryByRole("form", { name: "New comment" })).toBeNull()

  await act(async () => complete({
    ok: true,
    data: { owner, comments: [], permissions: allowed, next: "next-page" },
  }))
  expect(screen.queryByRole("form", { name: "New comment" })).toBeNull()

  jest.mocked(loadComments).mockResolvedValue({
    ok: true,
    data: { owner, comments: [], permissions: allowed },
  })
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByRole("form", { name: "New comment" })

  rendered.rerender(<Discussion owner={{ ...owner }} ownerPermissions={received} />)
  expect(screen.getByRole("form", { name: "New comment" })).toBeVisible()
})

test.each<DiscussionOwner>([owner, { kind: "page", id: 1, title: "Page", url: "/pages/page" }])(
  "empty $kind restoration still discovers new comments independently of initial permissions",
  async (discussion) => {
    const rendered = render(<Discussion owner={discussion} ownerPermissions={restricted} />)
    fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
    await waitFor(() => expect(screen.queryByRole("button", { name: "Load more comments" })).toBeNull())

    const saved = captureReadingPosition()
    rendered.unmount()
    prepareReadingPosition(saved)
    jest.mocked(loadComments).mockResolvedValue({
      ok: true,
      data: { owner: discussion, comments: [comment], permissions: allowed },
    })

    render(<Discussion owner={discussion} ownerPermissions={restricted} />)
    fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
    await screen.findByText(comment.content)

    expect(screen.getByRole("form", { name: "New comment" })).toBeVisible()
    expect(loadComments).toHaveBeenCalledTimes(2)
  }
)

test.each<DiscussionOwner>([owner, { kind: "page", id: 1, title: "Page", url: "/pages/page" }])(
  "a restricted $kind remains loadable until the server confirms it is empty",
  async (discussion) => {
    let complete!: (result: Awaited<ReturnType<typeof loadComments>>) => void
    jest.mocked(loadComments).mockReturnValueOnce(new Promise((resolve) => { complete = resolve }))

    const rendered = render(<Discussion owner={discussion} ownerPermissions={restricted} />)
    fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
    expect(screen.getByRole("region", { name: "Discussion" })).toBeVisible()

    await act(async () => complete({ ok: true, data: { owner: discussion, comments: [], permissions: restricted } }))
    expect(screen.queryByRole("region", { name: "Discussion" })).toBeNull()

    rendered.rerender(<Discussion owner={discussion} ownerPermissions={{ ...allowed }} />)
    expect(screen.getByRole("form", { name: "New comment" })).toBeVisible()
  }
)

test("restricted discussions keep existing comments without unavailable copy or a composer", async () => {
  jest.mocked(loadComments).mockResolvedValue({ ok: true, data: { owner, comments: [comment], permissions: restricted } })
  render(<Discussion owner={owner} ownerPermissions={restricted} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))

  await screen.findByText(comment.content)
  expect(screen.getByRole("region", { name: "Discussion" })).toBeVisible()
  expect(screen.queryByRole("form", { name: "New comment" })).toBeNull()
  expect(screen.queryByText("New comments are unavailable here.")).toBeNull()
})

test("a failed empty read retains retry and a successful retry can hide the discussion", async () => {
  jest.mocked(loadComments).mockResolvedValueOnce({ ok: false, status: 503, error: {} })
  render(<Discussion owner={owner} ownerPermissions={restricted} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByRole("alert")
  expect(screen.getByRole("region", { name: "Discussion" })).toBeVisible()

  fireEvent.click(screen.getByRole("button", { name: "Retry loading comments" }))
  await waitFor(() => expect(screen.queryByRole("region", { name: "Discussion" })).toBeNull())
})

test.each([allowed, { ...restricted, signInToComment: true }])(
  "an empty discussion retains the server-provided commenting action: $comment/$signInToComment",
  async (permissions) => {
    jest.mocked(loadComments).mockResolvedValue({ ok: true, data: { owner, comments: [], permissions } })
    render(<Discussion owner={owner} ownerPermissions={permissions} />)
    fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
    await waitFor(() => expect(screen.queryByRole("button", { name: "Load more comments" })).toBeNull())

    expect(screen.getByRole("region", { name: "Discussion" })).toBeVisible()
    expect(screen.queryByRole("form", { name: "New comment" }) !== null).toBe(permissions.comment)
    expect(screen.queryByRole("button", { name: "Sign in to comment" }) !== null).toBe(permissions.signInToComment)
  }
)

test("an empty page with a cursor remains loadable and a hidden discussion can open a permalink", async () => {
  jest.mocked(loadComments).mockResolvedValueOnce({
    ok: true, data: { owner, comments: [], permissions: restricted, next: "another-page" },
  })
  render(<Discussion owner={owner} ownerPermissions={restricted} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await waitFor(() => expect(loadComments).toHaveBeenCalledTimes(1))
  await waitFor(() => expect(screen.getByRole("button", { name: "Load more comments" })).toBeEnabled())
  expect(screen.getByRole("region", { name: "Discussion" })).toBeVisible()

  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await waitFor(() => expect(screen.queryByRole("region", { name: "Discussion" })).toBeNull())

  jest.mocked(loadCommentContext).mockResolvedValueOnce({
    ok: true, data: { owner, comments: [comment], permissions: restricted, commentId: comment.id },
  })
  window.history.replaceState(null, "", `${owner.url}#comment-${comment.id}`)
  fireEvent(window, new HashChangeEvent("hashchange"))
  await screen.findByText(comment.content)
  expect(screen.getByRole("region", { name: "Discussion" })).toBeVisible()
  expect(screen.queryByRole("form", { name: "New comment" })).toBeNull()
})
