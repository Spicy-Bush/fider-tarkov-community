import React from "react"
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { DiscussionComment, DiscussionOwner } from "@fider/models"
import { loadCommentRecords, loadComments } from "@fider/services/discussion"
import { captureReadingPosition, prepareReadingPosition } from "@fider/services/readingPosition"
import { Discussion } from "./Discussion"

jest.mock("@fider/hooks", () => ({ useFider: () => ({ session: { isAuthenticated: true } }) }))
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
}))

const owner = { kind: "post" as const, id: 1, number: 1, title: "Post", url: "/posts/1" }
const allowed = { comment: true, react: true, images: false }
const restricted = { comment: false, react: false, images: false }
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
    expect(screen.getByText("New comments are unavailable here.")).toBeVisible()
  }
  expect(screen.queryAllByRole("article")).toHaveLength(0)
  expect(loadComments).not.toHaveBeenCalled()
  expect(loadCommentRecords).not.toHaveBeenCalled()
})

test("a fresh parent receipt updates permissions while ordinary rerenders retain accepted reads", async () => {
  const rendered = render(<Discussion owner={owner} ownerPermissions={allowed} />)
  expect(screen.getByRole("form", { name: "New comment" })).toBeVisible()

  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByText("New comments are unavailable here.")

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
