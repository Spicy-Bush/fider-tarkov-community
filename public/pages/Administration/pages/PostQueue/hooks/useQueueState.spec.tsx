import { act, renderHook } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { Post } from "@fider/models"
import { actions } from "@fider/services"
import { useQueueState } from "./useQueueState"

jest.mock("@fider/hooks", () => ({
  useFider: () => ({ settings: { queueDefaultDate: "" }, session: { user: { id: 1 } } }),
}))
jest.mock("@fider/services", () => ({
  actions: { getPost: jest.fn(), getPostAttachments: jest.fn() },
  PAGINATION: { QUEUE_LIMIT: 20 },
}))

const original = {
  id: 1,
  number: 1,
  title: "Original",
  description: "Original content",
  tags: [],
  permissions: { edit: true },
  discussionPermissions: { comment: true, signInToComment: false, react: true, images: true },
} as Post

beforeEach(() => {
  jest.resetAllMocks()
  jest.mocked(actions.getPostAttachments).mockResolvedValue({ ok: true, data: [] })
})

test("an edit confirmed after refresh changes content without restoring captured permission metadata", async () => {
  const refreshed = {
    ...original,
    tags: ["fresh-tag"],
    permissions: { ...original.permissions, edit: false },
    discussionPermissions: { comment: false, signInToComment: false, react: false, images: true },
  }
  jest.mocked(actions.getPost).mockResolvedValue({ ok: true, data: refreshed })
  const { result } = renderHook(useQueueState)
  act(() => {
    result.current.selectPost(original)
    result.current.setPosts([original])
  })

  const pendingEdit = { ...original, title: "Saved title", description: "Saved content" }
  await act(async () => result.current.loadPostDetails(1, true))
  act(() => result.current.updatePost(pendingEdit))

  expect(result.current.selectedPost?.title).toBe("Saved title")
  expect(result.current.selectedPost?.description).toBe("Saved content")
  expect(result.current.selectedPost?.discussionPermissions).toBe(refreshed.discussionPermissions)
  expect(result.current.selectedPost?.permissions).toBe(refreshed.permissions)
  expect(result.current.selectedPost?.tags).toEqual(["fresh-tag"])
  expect(result.current.posts[0].permissions).toBe(refreshed.permissions)
  expect(result.current.posts[0].discussionPermissions).toBe(refreshed.discussionPermissions)
})

test("a late edit completion does not replace a different selected post", () => {
  const next = { ...original, id: 2, number: 2 }
  const { result } = renderHook(useQueueState)
  act(() => result.current.selectPost(next))
  act(() => result.current.updatePost({ id: 1, title: "Saved title", description: "Saved content" }))
  expect(result.current.selectedPost).toBe(next)
})
