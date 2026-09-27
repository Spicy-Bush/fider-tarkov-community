import React from "react"
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { DiscussionComment, DiscussionOwner, DiscussionPage } from "@fider/models"
import { loadCommentRecords, loadComments } from "@fider/services/discussion"
import { captureReadingPosition, finishReadingPosition, prepareReadingPosition, savedReadingPosition } from "@fider/services/readingPosition"
import { RequestError } from "@fider/services/http"
import { Discussion } from "./Discussion"

let mockAuthenticated = true

jest.mock("@fider/hooks", () => ({ useFider: () => ({ session: { isAuthenticated: mockAuthenticated } }) }))
jest.mock("@fider/components", () => ({
  Button: jest.requireActual("@fider/components/common/Button").Button,
  SignInModal: () => null,
}))
jest.mock("./CommentComposer", () => ({ CommentComposer: () => <form aria-label="New comment" /> }))
jest.mock("./DiscussionCommentCard", () => ({
  DiscussionCommentCard: ({ comment }: { comment: DiscussionComment }) => (
    <article id={`comment-${comment.id}`}>
      {comment.content}
      {comment.permissions.moderate && <button>Moderate {comment.id}</button>}
    </article>
  ),
}))
jest.mock("@fider/services/discussion", () => ({
  ...jest.requireActual("@fider/services/discussion"),
  loadComments: jest.fn(),
  loadCommentRecords: jest.fn(),
}))

const ownerPermissions = { comment: false, react: false, images: false }

const owner = { kind: "post" as const, id: 1, number: 1, title: "Post", url: "/posts/1" }
const comments: DiscussionComment[] = Array.from({ length: 3000 }, (_, index) => ({
  id: index + 1,
  parentId: null,
  hasReplies: false,
  content: `Previous content ${index + 1}`,
  createdAt: "2026-09-26T00:00:00Z",
  user: null,
  state: "visible",
  permissions: { edit: true, delete: true, moderate: true, react: true, reply: true, report: false },
}))
const page: DiscussionPage = { owner, comments, permissions: { comment: true, react: true, images: false } }

beforeEach(() => {
  jest.resetAllMocks()
  mockAuthenticated = true
  prepareReadingPosition(undefined)
  window.history.replaceState(null, "", owner.url)
  window.scrollBy = jest.fn()
  window.IntersectionObserver = jest.fn(() => ({ observe: jest.fn(), disconnect: jest.fn() })) as any
  window.ResizeObserver = jest.fn(() => ({ observe: jest.fn(), disconnect: jest.fn() })) as any
  jest.mocked(loadComments).mockResolvedValue({ ok: true, data: page })
})

test.each<DiscussionOwner>([
  owner,
  { kind: "page", id: 1, title: "Page", url: "/pages/page" },
])("returning to an empty $kind discussion reads current comments and permissions", async (discussion) => {
  mockAuthenticated = false
  jest.mocked(loadComments).mockResolvedValue({
    ok: true,
    data: { ...page, owner: discussion, comments: [], permissions: { ...page.permissions, comment: false } },
  })

  const rendered = render(<Discussion ownerPermissions={ownerPermissions} owner={discussion} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByRole("button", { name: "Sign in to comment" })

  const saved = captureReadingPosition()
  rendered.unmount()
  prepareReadingPosition(saved)
  mockAuthenticated = true
  jest.mocked(loadComments).mockResolvedValue({
    ok: true,
    data: { ...page, owner: discussion, comments: [comments[0]] },
  })

  render(<Discussion ownerPermissions={ownerPermissions} owner={discussion} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))

  await screen.findByText("Previous content 1")
  expect(screen.getByRole("form", { name: "New comment" })).toBeVisible()
  expect(screen.queryByRole("button", { name: "Sign in to comment" })).toBeNull()
  expect(loadComments).toHaveBeenCalledTimes(2)
  expect(loadCommentRecords).not.toHaveBeenCalled()
})

test("an empty discussion can recover a failed read with newly restricted permissions", async () => {
  jest.mocked(loadComments).mockResolvedValue({ ok: true, data: { ...page, comments: [] } })

  const rendered = render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByRole("form", { name: "New comment" })

  const saved = captureReadingPosition()
  rendered.unmount()
  prepareReadingPosition(saved)
  jest.mocked(loadComments).mockResolvedValue({ ok: false, status: 503, error: {} })

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByRole("alert")
  expect(screen.queryByRole("form", { name: "New comment" })).toBeNull()
  await finishReadingPosition(new AbortController().signal)

  jest.mocked(loadComments).mockResolvedValue({
    ok: true,
    data: { ...page, comments: [], permissions: { ...page.permissions, comment: false } },
  })
  fireEvent.click(screen.getByRole("button", { name: "Retry loading comments" }))

  await screen.findByText("New comments are unavailable here.")
  expect(screen.queryByRole("alert")).toBeNull()
  expect(screen.queryByRole("form", { name: "New comment" })).toBeNull()
})

async function leaveDiscussion() {
  const rendered = render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByText("Previous content 1")
  fireEvent.click(screen.getByRole("button", { name: "Collapse thread for comment 1" }))
  const saved = captureReadingPosition()
  rendered.unmount()
  prepareReadingPosition(saved)
  return saved
}

test("a formerly empty reply branch discovers replies after returning", async () => {
  const parent = { ...comments[0], hasReplies: true }
  jest.mocked(loadComments)
    .mockResolvedValueOnce({ ok: true, data: { ...page, comments: [parent] } })
    .mockResolvedValueOnce({ ok: true, data: { ...page, comments: [] } })
    .mockResolvedValueOnce({
      ok: true,
      data: { ...page, comments: [{ ...comments[1], parentId: parent.id, content: "New reply" }] },
    })

  const rendered = render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByText(parent.content)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await waitFor(() => expect(screen.queryByRole("button", { name: "Load more comments" })).toBeNull())

  const saved = captureReadingPosition()
  rendered.unmount()
  prepareReadingPosition(saved)
  jest.mocked(loadCommentRecords).mockResolvedValue({ ok: true, data: { ...page, comments: [parent] } })

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText(parent.content)
  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))

  await screen.findByText("New reply")
  expect(loadComments).toHaveBeenLastCalledWith(owner, "liked", parent.id, undefined, 4, expect.any(AbortSignal))
})

test("a large discussion retains its reading outline and refreshes only visible records", async () => {
  await leaveDiscussion()
  const saved = savedReadingPosition<{ comments: unknown[] }>("discussion:post:1")!
  expect(saved.comments).toHaveLength(3000)
  expect(JSON.stringify(saved)).not.toContain("Previous content")
  expect(JSON.stringify(saved)).not.toContain("permissions")

  jest.mocked(loadCommentRecords).mockImplementation(async (_owner, ids) => ({
    ok: true,
    data: {
      ...page,
      comments: ids.map((id) => ({
        ...comments[id - 1],
        content: `Current content ${id}`,
        permissions: { ...comments[id - 1].permissions, moderate: false },
      })),
    },
  }))

  render(<React.StrictMode><Discussion ownerPermissions={ownerPermissions} owner={owner} /></React.StrictMode>)
  await screen.findByText("Current content 1")
  await finishReadingPosition(new AbortController().signal)

  expect(loadCommentRecords).toHaveBeenCalledTimes(1)
  expect(jest.mocked(loadCommentRecords).mock.calls[0][1].length).toBeLessThan(30)
  expect(screen.getAllByRole("article").length).toBeLessThan(30)
  expect(screen.queryByRole("button", { name: "Moderate 1" })).toBeNull()
  expect(screen.getByRole("button", { name: "Expand thread for comment 1" })).toHaveAttribute("aria-expanded", "false")
  expect(loadComments).toHaveBeenCalledTimes(1)
})

test.each<[number, string | undefined]>([
  [3000, undefined],
  [3000, "saved-continuation"],
  [50000, undefined],
  [50000, "saved-continuation"],
])("restoring %i comments preserves traversal cursor %s", async (total, next) => {
  const outline = Array.from({ length: total }, (_, index) => ({
    id: index + 1,
    parentId: null,
    hasReplies: false,
    collapsed: false,
    pending: "unloaded",
  }))
  prepareReadingPosition({
    "discussion:post:1": {
      sort: "liked",
      comments: outline,
      branches: { 0: { ids: outline.map((comment) => comment.id), next } },
      collapsed: {},
      expanded: {},
      measurements: [],
      viewport: { top: total * 240 - 500, bottom: total * 240 + 500 },
    },
  })
  jest.mocked(loadCommentRecords).mockImplementation(async (_owner, ids) => ({
    ok: true,
    data: { ...page, comments: ids.map((id) => ({ ...comments[0], id, content: `Current comment ${id}` })) },
  }))
  jest.mocked(loadComments).mockResolvedValue({ ok: true, data: { ...page, comments: [] } })

  render(<Discussion owner={owner} ownerPermissions={ownerPermissions} />)
  await screen.findByText(`Current comment ${total}`)

  expect(loadCommentRecords).toHaveBeenCalledTimes(1)
  expect(jest.mocked(loadCommentRecords).mock.calls[0][1].length).toBeLessThan(30)

  if (next) {
    fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
    await waitFor(() => expect(loadComments).toHaveBeenCalledTimes(1))
    expect(loadComments).toHaveBeenCalledWith(owner, "liked", undefined, next, 5, expect.any(AbortSignal))
  } else {
    expect(screen.queryByRole("button", { name: "Load more comments" })).toBeNull()
    expect(loadComments).not.toHaveBeenCalled()
  }
})

test.each(["response", "transport"])("a %s failure stays retryable and does not hold the navigation open", async (failure) => {
  await leaveDiscussion()

  if (failure === "response") {
    jest.mocked(loadCommentRecords).mockResolvedValue({ ok: false, status: 503, error: { errors: [{ message: "Unavailable" }] } })
  } else {
    jest.mocked(loadCommentRecords).mockRejectedValue(new RequestError("GET", "/api/posts/1/comments", "transport", new Error("Offline")))
  }

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByRole("alert")
  await finishReadingPosition(new AbortController().signal)

  jest.mocked(loadCommentRecords).mockResolvedValue({ ok: true, data: page })
  fireEvent.click(screen.getByRole("button", { name: "Retry loading comments" }))
  await screen.findByText("Previous content 1")
  expect(screen.queryByRole("alert")).toBeNull()
  expect(window.scrollBy).not.toHaveBeenCalled()
})

test("a missing parent does not remove its already-loaded visible replies", async () => {
  prepareReadingPosition({
    "discussion:post:1": {
      sort: "liked",
      comments: [
        { id: 1, parentId: null, hasReplies: true, collapsed: false, pending: "unloaded" },
        { id: 2, parentId: 1, hasReplies: false, collapsed: false, pending: "unloaded" },
      ],
      branches: { 0: { ids: [1] }, 1: { ids: [2] } },
      collapsed: {},
      expanded: {},
      measurements: [],
    },
  })
  jest.mocked(loadCommentRecords).mockResolvedValue({
    ok: true,
    data: { ...page, comments: [{ ...comments[1], parentId: 1, content: "Visible reply" }] },
  })

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  await screen.findByText("Visible reply")

  expect(screen.getByText("Comment unavailable.")).toBeVisible()
  expect(screen.getByRole("button", { name: "Collapse thread for comment 1" })).toBeVisible()
})

test("canceling navigation releases its wait and unmount aborts record hydration", async () => {
  await leaveDiscussion()
  jest.mocked(loadCommentRecords).mockReturnValue(new Promise(() => {}))
  const rendered = render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  const navigation = new AbortController()
  const finished = finishReadingPosition(navigation.signal)

  await act(async () => navigation.abort())
  await finished
  rendered.unmount()

  expect(jest.mocked(loadCommentRecords).mock.calls[0][2].aborted).toBe(true)
})

test("a navigation canceled before publication does not leave a pending outline", async () => {
  await leaveDiscussion()
  const navigation = new AbortController()
  navigation.abort()

  await finishReadingPosition(navigation.signal)

  expect(savedReadingPosition("discussion:post:1")).toBeUndefined()
})

test("scrolling while records load keeps the user's new reading position", async () => {
  await leaveDiscussion()
  let complete!: (value: Awaited<ReturnType<typeof loadCommentRecords>>) => void
  jest.mocked(loadCommentRecords).mockReturnValue(new Promise((resolve) => {
    complete = resolve
  }))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  fireEvent.wheel(window)
  await finishReadingPosition(new AbortController().signal)
  await act(async () => complete({ ok: true, data: page }))

  await screen.findByText("Previous content 1")
  expect(window.scrollBy).not.toHaveBeenCalled()
})

test("viewport changes finish the pending batch before loading newly visible comments", async () => {
  await leaveDiscussion()
  let complete!: (value: Awaited<ReturnType<typeof loadCommentRecords>>) => void
  jest.mocked(loadCommentRecords)
    .mockReturnValueOnce(new Promise((resolve) => {
      complete = resolve
    }))
    .mockImplementation(async (_owner, ids) => ({
      ok: true,
      data: { ...page, comments: ids.map((id) => comments[id - 1]) },
    }))

  const height = window.innerHeight
  const rendered = render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  const [, requested, signal] = jest.mocked(loadCommentRecords).mock.calls[0]

  try {
    for (const nextHeight of [300, 2400]) {
      await act(async () => {
        window.innerHeight = nextHeight
        fireEvent.resize(window)
        await new Promise(requestAnimationFrame)
      })

      expect(signal.aborted).toBe(false)
      expect(loadCommentRecords).toHaveBeenCalledTimes(1)
    }

    await act(async () => complete({
      ok: true,
      data: { ...page, comments: requested.map((id) => comments[id - 1]) },
    }))

    await waitFor(() => expect(loadCommentRecords).toHaveBeenCalledTimes(2))
    const additional = jest.mocked(loadCommentRecords).mock.calls[1][1]
    expect(additional.length).toBeGreaterThan(0)
    expect(additional.some((id) => requested.includes(id))).toBe(false)
    await screen.findByText(`Previous content ${additional[0]}`)
  } finally {
    rendered.unmount()
    window.innerHeight = height
  }
})

test("changing sort cancels hydration without requesting the old selection again", async () => {
  await leaveDiscussion()
  jest.mocked(loadCommentRecords).mockReturnValue(new Promise(() => {}))

  render(<Discussion ownerPermissions={ownerPermissions} owner={owner} />)
  const signal = jest.mocked(loadCommentRecords).mock.calls[0][2]

  await act(async () => {
    fireEvent.change(screen.getByRole("combobox", { name: "Sort discussion" }), { target: { value: "latest" } })
    await new Promise(requestAnimationFrame)
  })

  expect(signal.aborted).toBe(true)
  expect(loadCommentRecords).toHaveBeenCalledTimes(1)

  fireEvent.click(screen.getByRole("button", { name: "Load more comments" }))
  await screen.findByText("Previous content 1")
  expect(loadComments).toHaveBeenLastCalledWith(owner, "latest", undefined, undefined, 5, expect.any(AbortSignal))
  expect(loadCommentRecords).toHaveBeenCalledTimes(1)
})
