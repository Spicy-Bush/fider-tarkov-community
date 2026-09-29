import { http, Result, querystring } from "@fider/services"
import { Page, PageTag, PageTopic } from "@fider/models"

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
  return http.get<SearchPagesResult>(`/api/pages${qs}`)
}


export const deletePage = async (id: number): Promise<Result> => {
  return http.delete(`/api/pages/${id}`)
}

export const togglePageReaction = async (id: number, emoji: string): Promise<Result<{ added: boolean }>> => {
  return http.post<{ added: boolean }>(`/api/pages/${id}/reactions`, { emoji })
}

export const togglePageSubscription = async (id: number): Promise<Result<{ subscribed: boolean }>> => {
  return http.post<{ subscribed: boolean }>(`/api/pages/${id}/subscribe`, {})
}

export const createPageTopic = async (input: {
  name: string
  slug?: string
  description?: string
  color?: string
}): Promise<Result<PageTopic>> => {
  return http.post<PageTopic>("/api/page-topics", input)
}

export const updatePageTopic = async (
  id: number,
  input: { name: string; slug?: string; description?: string; color?: string }
): Promise<Result> => {
  return http.put(`/api/page-topics/${id}`, input)
}

export const deletePageTopic = async (id: number): Promise<Result> => {
  return http.delete(`/api/page-topics/${id}`)
}

export const createPageTag = async (input: { name: string; slug?: string }): Promise<Result<PageTag>> => {
  return http.post<PageTag>("/api/page-tags", input)
}

export const updatePageTag = async (id: number, input: { name: string; slug?: string }): Promise<Result> => {
  return http.put(`/api/page-tags/${id}`, input)
}

export const deletePageTag = async (id: number): Promise<Result> => {
  return http.delete(`/api/page-tags/${id}`)
}
