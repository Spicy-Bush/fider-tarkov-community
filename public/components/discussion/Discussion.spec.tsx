import { test, expect } from "@jest/globals"
import { DiscussionComment } from "@fider/models"
import { discussionRows } from "./Discussion"
import { isNegativelyRated } from "@fider/services/discussion"

jest.mock("./CommentComposer", () => ({ CommentComposer: () => null }))
jest.mock("./DiscussionCommentCard", () => ({ DiscussionCommentCard: () => null }))

function comment(id: number, parentId: number | null = null): DiscussionComment {
  return {
    id,
    parentId,
    createdAt: "2026-09-26T00:00:00Z",
    content: `Comment ${id}`,
    user: { id: 1, name: "Visitor", role: "visitor" },
    state: "visible",
    hasReplies: false,
    permissions: { edit: false, delete: false, moderate: false, reply: true, react: true, report: true },
  } as DiscussionComment
}

test.each([
  [0, 1, false],
  [0, 3, false],
  [0, 4, true],
  [0, 5, true],
  [1, 4, false],
  [1, 5, true],
  [6, 2, false],
])("%i likes and %i dislikes set automatic collapse to %s", (likes, dislikes, collapsed) => {
  const rated = comment(1)
  rated.reactionCounts = [
    { emoji: "👍", count: likes, includesMe: false },
    { emoji: "👎", count: dislikes, includesMe: false },
  ]

  expect(isNegativelyRated(rated)).toBe(collapsed)
})

test.each(["helper", "moderator", "collaborator", "administrator"] as const)("%s comments stay open at a negative score", (role) => {
  const rated = comment(1)
  rated.user!.role = role
  rated.reactionCounts = [{ emoji: "👎", count: 100, includesMe: false }]

  expect(isNegativelyRated(rated)).toBe(false)
})

test("thread spans include descendants and pending pages but exclude siblings", () => {
  const state = {
    comments: { 1: comment(1), 2: comment(2, 1), 3: comment(3, 2), 4: comment(4) },
    branches: {
      0: { ids: [1, 4], page: {} },
      1: { ids: [2], page: { next: "more-replies" } },
      2: { ids: [3], page: {} },
    },
  }

  expect(discussionRows(state, {})).toEqual([
    { kind: "comment", id: 1, depth: 0, end: 4, collapsed: false },
    { kind: "comment", id: 2, depth: 1, end: 3, collapsed: false },
    { kind: "comment", id: 3, depth: 2, end: 3, collapsed: false },
    { kind: "load", parentId: 1, depth: 1, levels: 4 },
    { kind: "comment", id: 4, depth: 0, end: 5, collapsed: false },
  ])

  expect(discussionRows(state, { 2: true })).toEqual([
    { kind: "comment", id: 1, depth: 0, end: 3, collapsed: false },
    { kind: "comment", id: 2, depth: 1, end: 2, collapsed: true },
    { kind: "load", parentId: 1, depth: 1, levels: 4 },
    { kind: "comment", id: 4, depth: 0, end: 4, collapsed: false },
  ])
})

test("a manual expansion reveals a negatively rated parent and its staff reply", () => {
  const root = comment(1)
  root.reactionCounts = [{ emoji: "👎", count: 4, includesMe: false }]
  const reply = comment(2, 1)
  reply.user!.role = "administrator"

  const state = {
    comments: { 1: root, 2: reply },
    branches: { 0: { ids: [1], page: {} }, 1: { ids: [2], page: {} } },
  }

  expect(discussionRows(state, {})).toHaveLength(1)
  expect(discussionRows(state, { 1: false })).toHaveLength(2)
  expect(discussionRows(state, { 1: true })).toHaveLength(1)
})

test("deep replies reveal four levels followed by ten more per expansion", () => {
  const comments: Record<number, DiscussionComment> = {}
  const branches: Record<number, { ids: number[]; page: { next?: string } }> = { 0: { ids: [1], page: {} } }

  for (let id = 1; id <= 1000; id++) {
    comments[id] = comment(id, id > 1 ? id - 1 : null)
    branches[id] = { ids: id < 1000 ? [id + 1] : [], page: {} }
  }

  const rows = discussionRows({ comments, branches }, {})
  expect(rows).toHaveLength(6)
  expect(rows[5]).toEqual({ kind: "continue", parentId: 5, depth: 4 })

  const focused = discussionRows({ comments, branches }, {}, 4)
  expect(focused[0]).toEqual({ kind: "comment", id: 4, depth: 0, end: 6, collapsed: false })
  expect(focused[5]).toEqual({ kind: "continue", parentId: 8, depth: 4 })

  const expandedOnce = discussionRows({ comments, branches }, {}, undefined, { 5: true })
  expect(expandedOnce).toHaveLength(16)
  expect(expandedOnce[15]).toEqual({ kind: "continue", parentId: 15, depth: 14 })

  const expandedTwice = discussionRows({ comments, branches }, {}, undefined, { 5: true, 15: true })
  expect(expandedTwice).toHaveLength(26)
  expect(expandedTwice[25]).toEqual({ kind: "continue", parentId: 25, depth: 24 })

  const collapsed = discussionRows({ comments, branches }, { 5: true }, undefined, { 5: true, 15: true })
  expect(collapsed).toHaveLength(5)
})
