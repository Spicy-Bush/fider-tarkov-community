import type { PendingPostSubmission } from "@fider/services/postSubmission"
import { http } from "@fider/services/http"
import * as querystring from "@fider/services/querystring"
import { Result } from "@fider/services"
import { Post, Vote, ImageUpload, UserNames, DiscussionComment } from "@fider/models"
import { retryRequest, RetryResult } from "@fider/services/retryRequest"

export const getAllPosts = async (): Promise<Result<Post[]>> => {
  return await http.get<Post[]>("/api/posts")
}

export const getPost = async (postNumber: number): Promise<Result<Post>> => {
  return await http.get<Post>(`/api/posts/${postNumber}`)
}

export const getPostAttachments = async (postNumber: number): Promise<Result<string[]>> => {
  return await http.get<string[]>(`/api/posts/${postNumber}/attachments`)
}

export interface SearchPostsParams {
  ids?: number[]
  query?: string
  view?: string
  limit?: number
  offset?: number
  tags?: string[]
  myVotes?: boolean
  myPosts?: boolean
  notMyVotes?: boolean
  statuses?: string[]
  date?: string
  tagLogic?: "OR" | "AND"
  includeCount?: boolean
}

export const searchPosts = async (
  params: SearchPostsParams,
  options?: { signal?: AbortSignal; notifyOnError?: boolean }
): Promise<Result<Post[]>> => {
  let qsParams = querystring.stringify({
    ids: params.ids?.join(","),
    tags: params.tags,
    statuses: params.statuses,
    query: params.query,
    view: params.view,
    limit: params.limit,
    offset: params.offset,
    date: params.date,
    tagLogic: params.tagLogic
  })
  if (params.myVotes) {
    qsParams += `&myvotes=true`
  }
  if (params.myPosts) {
    qsParams += `&myposts=true`
  }
  if (params.notMyVotes) {
    qsParams += `&notmyvotes=true`
  }
  if (params.includeCount) {
    qsParams += `&includeCount=true`
  }

  return await http.get<Post[]>(`/api/posts${qsParams}`, { ...options, includeHeaders: params.includeCount })
}

export const deletePost = async (postNumber: number, text: string): Promise<Result> => {
  return http
    .delete(`/api/posts/${postNumber}`, {
      text,
    })
    .then(http.event("post", "delete"))
}

export interface VoteState {
  direction: number
  revision: number
  upvotes: number
  downvotes: number
  applied: boolean
  lastActivityAt: string
}

export const setVote = async (postNumber: number, direction: number, revision: number): Promise<RetryResult<VoteState>> => {
  const input = { revision }
  return retryRequest(() => direction === 0
    ? http.delete<VoteState>(`/api/posts/${postNumber}/votes`, input)
    : http.post<VoteState>(`/api/posts/${postNumber}/${direction === 1 ? "up" : "down"}`, input),
  { attempts: 2, delayMs: 0 })
}

export const subscribe = async (postNumber: number): Promise<Result> => {
  return http.post(`/api/posts/${postNumber}/subscription`).then(http.event("post", "subscribe"))
}

export const unsubscribe = async (postNumber: number): Promise<Result> => {
  return http.delete(`/api/posts/${postNumber}/subscription`).then(http.event("post", "unsubscribe"))
}

export const listVotes = async (postNumber: number): Promise<Result<Vote[]>> => {
  return http.get<Vote[]>(`/api/posts/${postNumber}/votes`)
}

export const getTaggableUsers = async (nameFilter: string): Promise<Result<UserNames[]>> => {
  return http.get<UserNames[]>(`/api/taggable-users${querystring.stringify({ name: nameFilter })}`)
}

export const lockPost = async (postNumber: number, message: string): Promise<Result> => {
  return await http.put(`/api/posts/${postNumber}/lock`, { message });
};

export const unlockPost = async (postNumber: number): Promise<Result> => {
  return await http.delete(`/api/posts/${postNumber}/lock`);
};

interface SetResponseInput {
  status: string
  text: string
  originalNumber: number
}

export const respond = async (postNumber: number, input: SetResponseInput): Promise<Result> => {
  return http
    .put(`/api/posts/${postNumber}/status`, {
      status: input.status,
      text: input.text,
      originalNumber: input.originalNumber,
    })
    .then(http.event("post", "respond"))
}

interface CreatePostResponse {
  id: number
  number: number
  title: string
  slug: string
}

export const createPost = async (submission: PendingPostSubmission, signal: AbortSignal): Promise<Result<CreatePostResponse>> => {
  const { submissionId, title, description, attachments } = submission

  return http.post<CreatePostResponse>("/api/posts", { submissionId, title, description, attachments }, { signal })
}

export const updatePost = async (postNumber: number, title: string, description: string, attachments: ImageUpload[]): Promise<Result> => {
  return http.put(`/api/posts/${postNumber}`, { title, description, attachments }).then(http.event("post", "update"))
}

export const archivePost = async (postNumber: number): Promise<Result> => {
  return http.post(`/api/posts/${postNumber}/archive`).then(http.event("post", "archive"))
}

export const unarchivePost = async (postNumber: number): Promise<Result> => {
  return http.post(`/api/posts/${postNumber}/unarchive`).then(http.event("post", "unarchive"))
}

export interface GetArchivablePostsParams {
  page?: number
  perPage?: number
  createdBefore?: string
  inactiveSince?: string
  maxVotes?: number
  maxComments?: number
  statuses?: string[]
  tags?: string[]
}

export interface GetArchivablePostsResponse {
  posts: Post[]
  total: number
}

export const getArchivablePosts = async (params: GetArchivablePostsParams): Promise<Result<GetArchivablePostsResponse>> => {
  const qs = querystring.stringify({
    page: params.page,
    perPage: params.perPage,
    createdBefore: params.createdBefore,
    inactiveSince: params.inactiveSince,
    maxVotes: params.maxVotes,
    maxComments: params.maxComments,
    statuses: params.statuses,
    tags: params.tags,
  })
  return http.get<GetArchivablePostsResponse>(`/api/archive/posts${qs}`)
}

export const bulkArchivePosts = async (postIds: number[]): Promise<Result<{ archived: number }>> => {
  return http.post<{ archived: number }>(`/api/archive/bulk`, { postIds })
}

export const hidePost = async (postId: number): Promise<Result> => {
  return http.post(`/api/admin/moderation/posts/${postId}/hide`)
}

export const unhidePost = async (postId: number): Promise<Result> => {
  return http.post(`/api/admin/moderation/posts/${postId}/approve`)
}

export const hideComment = async (commentId: number): Promise<Result<DiscussionComment>> => {
  return http.post<DiscussionComment>(`/api/admin/moderation/comments/${commentId}/hide`)
}

export const unhideComment = async (commentId: number): Promise<Result<DiscussionComment>> => {
  return http.post<DiscussionComment>(`/api/admin/moderation/comments/${commentId}/approve`)
}
