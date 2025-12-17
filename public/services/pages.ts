import { http, Result, querystring } from "@fider/services"
import { Page, PageDraft, PageTag, PageTopic } from "@fider/models"
import { Comment } from "@fider/models"

export interface SearchPagesParams {
  q?: string
  topics?: string
  tags?: string[]
  view?: string
  page?: number
}

export interface SearchPagesResult {
  pages: Page[]
  totalCount: number
  totalPages: number
  page: number
}

export const searchPages = async (params: SearchPagesParams): Promise<Result<SearchPagesResult>> => {
  const qs = querystring.stringify({
    q: params.q,
    topics: params.topics,
    tags: params.tags?.join(","),
    view: params.view,
    page: params.page,
  })
  return http.get<SearchPagesResult>(`/api/v1/pages${qs}`)
}

export interface CreatePageInput {
  title: string
  slug?: string
  content: string
  excerpt?: string
  bannerImage?: any
  status: string
  visibility: string
  allowedRoles?: string[]
  parentPageId?: number
  allowComments?: boolean
  allowReactions?: boolean
  showToc?: boolean
  scheduledFor?: string
  authors?: number[]
  topics?: number[]
  tags?: number[]
  metaDescription?: string
  canonicalUrl?: string
}

export interface UpdatePageInput extends CreatePageInput {
  id: number
}

export const createPage = async (input: CreatePageInput): Promise<Result<Page>> => {
  return http.post<Page>("/_api/pages", input)
}

export const updatePage = async (id: number, input: UpdatePageInput): Promise<Result<Page>> => {
  return http.put<Page>(`/_api/pages/${id}`, input)
}

export const deletePage = async (id: number): Promise<Result> => {
  return http.delete(`/_api/pages/${id}`)
}

export const savePageDraft = async (id: number, draft: Partial<PageDraft>): Promise<Result> => {
  return http.post(`/_api/pages/${id}/draft`, draft)
}

export const getPageDraft = async (id: number): Promise<Result<PageDraft>> => {
  return http.get<PageDraft>(`/_api/pages/${id}/draft`)
}

export const togglePageReaction = async (id: number, emoji: string): Promise<Result<{ added: boolean }>> => {
  return http.post<{ added: boolean }>(`/api/v1/pages/${id}/reactions`, { emoji })
}

export const togglePageSubscription = async (id: number): Promise<Result<{ subscribed: boolean }>> => {
  return http.post<{ subscribed: boolean }>(`/api/v1/pages/${id}/subscribe`, {})
}

export const getPageComments = async (id: number): Promise<Result<Comment[]>> => {
  return http.get<Comment[]>(`/api/v1/pages/${id}/comments`)
}

export const addPageComment = async (id: number, content: string): Promise<Result<Comment>> => {
  return http.post<Comment>(`/api/v1/pages/${id}/comments`, { content })
}

export const togglePageCommentReaction = async (pageId: number, commentId: number, emoji: string): Promise<Result<{ added: boolean }>> => {
  return http.post<{ added: boolean }>(`/api/v1/pages/${pageId}/comments/${commentId}/reactions/${encodeURIComponent(emoji)}`, {})
}

export const createPageTopic = async (input: {
  name: string
  slug?: string
  description?: string
  color?: string
}): Promise<Result<PageTopic>> => {
  return http.post<PageTopic>("/_api/page-topics", input)
}

export const updatePageTopic = async (
  id: number,
  input: { name: string; slug?: string; description?: string; color?: string }
): Promise<Result> => {
  return http.put(`/_api/page-topics/${id}`, input)
}

export const deletePageTopic = async (id: number): Promise<Result> => {
  return http.delete(`/_api/page-topics/${id}`)
}

export const createPageTag = async (input: { name: string; slug?: string }): Promise<Result<PageTag>> => {
  return http.post<PageTag>("/_api/page-tags", input)
}

export const updatePageTag = async (id: number, input: { name: string; slug?: string }): Promise<Result> => {
  return http.put(`/_api/page-tags/${id}`, input)
}

export const deletePageTag = async (id: number): Promise<Result> => {
  return http.delete(`/_api/page-tags/${id}`)
}

