import { http, Result, querystring } from "@fider/services"
import { Post, Vote, ImageUpload, UserNames, Comment } from "@fider/models"

export const getAllPosts = async (): Promise<Result<Post[]>> => {
  return await http.get<Post[]>("/api/v1/posts")
}

export const getPost = async (postNumber: number): Promise<Result<Post>> => {
  return await http.get<Post>(`/api/v1/posts/${postNumber}`)
}

export const getAllComments = async (postNumber: number): Promise<Result<Comment[]>> => {
  return await http.get<Comment[]>(`/api/v1/posts/${postNumber}/comments`)
}

export const getPostAttachments = async (postNumber: number): Promise<Result<string[]>> => {
  return await http.get<string[]>(`/api/v1/posts/${postNumber}/attachments`)
}

export interface SearchPostsParams {
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

export const searchPosts = async (params: SearchPostsParams): Promise<Result<Post[]>> => {
  let qsParams = querystring.stringify({
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
    return await http.getWithHeaders<Post[]>(`/api/v1/posts${qsParams}`)
  }
  return await http.get<Post[]>(`/api/v1/posts${qsParams}`)
}

export const deletePost = async (postNumber: number, text: string): Promise<Result> => {
  return http
    .delete(`/api/v1/posts/${postNumber}`, {
      text,
    })
    .then(http.event("post", "delete"))
}

export const addVote = async (postNumber: number): Promise<Result> => {
  return http.post(`/api/v1/posts/${postNumber}/up`).then(http.event("post", "upvote"))
}

export const addDownVote = async (postNumber: number): Promise<Result> => {
  return http.post(`/api/v1/posts/${postNumber}/down`).then(http.event("post", "downvote"))
}

export const removeVote = async (postNumber: number): Promise<Result> => {
  return http.delete(`/api/v1/posts/${postNumber}/votes`).then(http.event("post", "unvote"))
}

export const toggleVote = async (postNumber: number, _direction?: "up" | "down"): Promise<Result> => {
  return http.post(`/api/v1/posts/${postNumber}/votes/toggle`).then(http.event("post", "toggle-vote"))
}

export const subscribe = async (postNumber: number): Promise<Result> => {
  return http.post(`/api/v1/posts/${postNumber}/subscription`).then(http.event("post", "subscribe"))
}

export const unsubscribe = async (postNumber: number): Promise<Result> => {
  return http.delete(`/api/v1/posts/${postNumber}/subscription`).then(http.event("post", "unsubscribe"))
}

export const listVotes = async (postNumber: number): Promise<Result<Vote[]>> => {
  return http.get<Vote[]>(`/api/v1/posts/${postNumber}/votes`)
}

export const getTaggableUsers = async (nameFilter: string): Promise<Result<UserNames[]>> => {
  return http.get<UserNames[]>(`/api/v1/taggable-users${querystring.stringify({ name: nameFilter })}`)
}

export const createComment = async (postNumber: number, content: string, attachments: ImageUpload[]): Promise<Result<{ id: number; attachments: string[] }>> => {
  return http.post<{ id: number; attachments: string[] }>(`/api/v1/posts/${postNumber}/comments`, { content, attachments }).then(http.event("comment", "create"))
}

export const updateComment = async (postNumber: number, commentID: number, content: string, attachments: ImageUpload[]): Promise<Result> => {
  return http.put(`/api/v1/posts/${postNumber}/comments/${commentID}`, { content, attachments }).then(http.event("comment", "update"))
}

export const deleteComment = async (postNumber: number, commentID: number): Promise<Result> => {
  return http.delete(`/api/v1/posts/${postNumber}/comments/${commentID}`).then(http.event("comment", "delete"))
}
interface ToggleReactionResponse {
  added: boolean
}

export const toggleCommentReaction = async (postNumber: number, commentID: number, emoji: string): Promise<Result<ToggleReactionResponse>> => {
  return http.post<ToggleReactionResponse>(`/api/v1/posts/${postNumber}/comments/${commentID}/reactions/${emoji}`)
}

export const lockPost = async (postNumber: number, message: string): Promise<Result> => {
  return await http.put(`/api/v1/posts/${postNumber}/lock`, { message });
};

export const unlockPost = async (postNumber: number): Promise<Result> => {
  return await http.delete(`/api/v1/posts/${postNumber}/lock`);
};

interface SetResponseInput {
  status: string
  text: string
  originalNumber: number
}

export const respond = async (postNumber: number, input: SetResponseInput): Promise<Result> => {
  return http
    .put(`/api/v1/posts/${postNumber}/status`, {
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

export const createPost = async (title: string, description: string, attachments: ImageUpload[]): Promise<Result<CreatePostResponse>> => {
  return http.post<CreatePostResponse>(`/api/v1/posts`, { title, description, attachments }).then(http.event("post", "create"))
}

export const updatePost = async (postNumber: number, title: string, description: string, attachments: ImageUpload[]): Promise<Result> => {
  return http.put(`/api/v1/posts/${postNumber}`, { title, description, attachments }).then(http.event("post", "update"))
}

export const archivePost = async (postNumber: number): Promise<Result> => {
  return http.post(`/api/v1/posts/${postNumber}/archive`).then(http.event("post", "archive"))
}

export const unarchivePost = async (postNumber: number): Promise<Result> => {
  return http.post(`/api/v1/posts/${postNumber}/unarchive`).then(http.event("post", "unarchive"))
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
  return http.get<GetArchivablePostsResponse>(`/api/v1/archive/posts${qs}`)
}

export const bulkArchivePosts = async (postIds: number[]): Promise<Result<{ archived: number }>> => {
  return http.post<{ archived: number }>(`/api/v1/archive/bulk`, { postIds })
}

export const hidePost = async (postId: number): Promise<Result> => {
  return http.post(`/_api/admin/moderation/posts/${postId}/hide`)
}

export const unhidePost = async (postId: number): Promise<Result> => {
  return http.post(`/_api/admin/moderation/posts/${postId}/approve`)
}

export const hideComment = async (commentId: number): Promise<Result> => {
  return http.post(`/_api/admin/moderation/comments/${commentId}/hide`)
}

export const unhideComment = async (commentId: number): Promise<Result> => {
  return http.post(`/_api/admin/moderation/comments/${commentId}/approve`)
}
