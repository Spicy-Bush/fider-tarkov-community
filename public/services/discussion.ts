import { CommentContext, DiscussionComment, DiscussionOwner, DiscussionPage, DiscussionSort } from "../models/discussion"
import { ImageUpload } from "@fider/models"
import { http } from "./http"
import { retryRequest } from "./retryRequest"

export interface CommentSubmission {
  submissionId: string
  content: string
  attachments: ImageUpload[]
  parentId?: number
}

export function discussionURL(owner: DiscussionOwner): string {
  return owner.kind === "post"
    ? `/api/posts/${owner.number}/comments`
    : `/api/pages/${owner.id}/comments`
}

export function loadComments(
  owner: DiscussionOwner,
  sort: DiscussionSort,
  parentId?: number,
  cursor?: string,
  depth = 1,
  signal?: AbortSignal
) {
  const parameters = new URLSearchParams({ sort })

  if (parentId !== undefined) {
    parameters.set("parentId", String(parentId))
    parameters.set("depth", String(depth))
  }

  if (cursor) {
    parameters.set("after", cursor)
  }

  return http.get<DiscussionPage>(`${discussionURL(owner)}?${parameters}`, { notifyOnError: false, signal })
}

export function loadCommentContext(id: number, signal?: AbortSignal) {
  return http.get<CommentContext>(`/api/comments/${id}`, { notifyOnError: false, signal })
}

export function loadCommentRecords(owner: DiscussionOwner, ids: number[], signal: AbortSignal) {
  const parameters = new URLSearchParams({ ids: ids.join(",") })
  return http.get<DiscussionPage>(`${discussionURL(owner)}?${parameters}`, { notifyOnError: false, signal })
}

export function submitComment(owner: DiscussionOwner, submission: CommentSubmission, signal?: AbortSignal) {
  return retryRequest(() => http.post<DiscussionComment>(discussionURL(owner), submission, { notifyOnError: false, signal }), { signal })
}

export function editComment(id: number, submission: CommentSubmission, signal?: AbortSignal) {
  return retryRequest(() => http.put<DiscussionComment>(`/api/comments/${id}`, submission, { notifyOnError: false, signal }), { signal })
}

export function deleteComment(id: number) {
  return retryRequest(() => http.delete<DiscussionComment>(`/api/comments/${id}`, undefined, { notifyOnError: false }))
}

export function setCommentReaction(id: number, emoji: string, active: boolean) {
  return retryRequest(() => http.put<Pick<DiscussionComment, "reactionCounts">>(
    `/api/comments/${id}/reactions/${encodeURIComponent(emoji)}`,
    { active },
    { notifyOnError: false }
  ))
}

export function isNegativelyRated(comment: DiscussionComment): boolean {
  if (!comment.user || comment.user.role !== "visitor") {
    return false
  }

  const positive = comment.reactionCounts?.find((reaction) => reaction.emoji === "👍")?.count || 0
  const negative = comment.reactionCounts?.find((reaction) => reaction.emoji === "👎")?.count || 0
  return negative - positive >= 4
}
