import React from "react"
import { fireEvent, render, screen } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { i18n } from "@lingui/core"
import { DiscussionComment, Post, PostStatusValue, Report, User, UserRole, UserStatus } from "@fider/models"
import { Fider } from "@fider/services"
import { noSessionPermissions, noUserPermissions } from "@fider/services/testing/permissions"
import { ContentPreview } from "./pages/ManageReports/components/ContentPreview"
import { PostViewer } from "./pages/PostQueue/components/PostViewer"

jest.mock("@fider/pages/ShowPost/components/VoteSection", () => ({ VoteSection: () => null }))
jest.mock("@fider/pages/ShowPost/components/TagsPanel", () => ({ TagsPanel: () => null }))
jest.mock("@fider/pages/ShowPost/components/DiscussionPanel", () => ({ DiscussionPanel: () => null }))
jest.mock("./pages/PostQueue/components/PostQueueActions", () => ({ PostQueueActions: () => null }))

const author: User = {
  id: 7,
  name: "Reported author",
  avatarURL: "",
  role: UserRole.Visitor,
  status: UserStatus.Active,
  permissions: { ...noUserPermissions, readProfile: true, moderate: true },
}

const reporter: User = {
  ...author,
  id: 8,
  name: "Reporter",
  role: UserRole.Administrator,
  permissions: { ...noUserPermissions, readProfile: true },
}

const post: Post = {
  id: 1,
  number: 1,
  slug: "reported-post",
  title: "Reported post",
  description: "Post content",
  createdAt: "2026-01-01T00:00:00Z",
  lastActivityAt: "2026-01-01T00:00:00Z",
  status: PostStatusValue.Open,
  user: author,
  voteType: 0,
  voteRevision: 0,
  response: null,
  votesCount: 0,
  commentsCount: 0,
  tags: [],
  discussionPermissions: { comment: false, react: false, images: false },
  permissions: {
    edit: false, delete: false, respond: [], lock: false, archive: false, moderate: false,
    tag: false, report: false, viewVotes: false, vote: false, follow: false,
  },
}

const comment: DiscussionComment = {
  id: 2,
  content: "Comment content",
  createdAt: post.createdAt,
  user: author,
  parentId: null,
  hasReplies: false,
  state: "visible",
  permissions: { edit: false, delete: false, moderate: false, reply: false, react: false, report: false },
}

beforeEach(() => {
  i18n.load("en", {})
  i18n.activate("en")
  Fider.initialize({ permissions: noSessionPermissions, user: reporter, contextID: "profile-links", tenant: {}, settings: {} })
})

test.each(["post", "comment"] as const)("a %s report preserves each profile's projected permissions", (reportedType) => {
  const report: Report = {
    id: 1,
    reportedType,
    reportedId: reportedType === "post" ? post.id : comment.id,
    reason: "Spam",
    status: "pending",
    createdAt: post.createdAt,
    reporter,
  }
  const openProfile = jest.fn()
  render(
    <ContentPreview
      report={report}
      post={post}
      comment={comment}
      isLoading={false}
      currentUserId={reporter.id}
      onAssign={() => {}}
      onUnassign={() => {}}
      onResolve={() => {}}
      onUserClick={openProfile}
    />,
  )

  fireEvent.click(screen.getByRole("button", { name: /Reported author/ }))
  expect(openProfile.mock.calls[0][0]).toBe(author)
  expect(openProfile.mock.calls[0][0].permissions.moderate).toBe(true)

  fireEvent.click(screen.getByRole("button", { name: /Reporter/ }))
  expect(openProfile.mock.calls[1][0]).toBe(reporter)
  expect(openProfile.mock.calls[1][0].permissions.moderate).toBe(false)
})

test("the queue author link preserves projected profile permissions", () => {
  const openProfile = jest.fn()
  render(
    <PostViewer
      post={post}
      tags={[]}
      attachments={[]}
      isLoading={false}
      allTags={[]}
      showDuplicateSearch={false}
      duplicateOriginalNumber={0}
      onShowDuplicateSearch={() => {}}
      onDuplicateSelected={() => {}}
      onDuplicateCancelled={() => {}}
      onDuplicateReset={() => {}}
      onPostUpdated={() => {}}
      onContentCopied={() => {}}
      onUserClick={openProfile}
    />,
  )

  fireEvent.click(screen.getByRole("button", { name: /Reported author/ }))
  expect(openProfile.mock.calls[0][0]).toBe(author)
  expect(openProfile.mock.calls[0][0].permissions.moderate).toBe(true)
})
