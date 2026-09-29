import { http, Result } from "@fider/services/http"
import { Notification } from "@fider/models"

export interface PaginatedNotifications {
  notifications: Notification[]
  total: number
  unreadTotal: number
  readTotal: number
  page: number
  perPage: number
}

export interface UnreadCountsResponse {
  total: number
  pendingReports?: number
  queueCount?: number
}

export const getUnreadCounts = async (options?: { signal?: AbortSignal }): Promise<Result<UnreadCountsResponse>> => {
  return http.get<UnreadCountsResponse>("/api/notifications/unread/total", options)
}

export const getNotifications = async (
  page: number = 1, 
  perPage: number = 10, 
  type?: string,
  options?: { signal?: AbortSignal }
): Promise<Result<PaginatedNotifications>> => {
  let url = `/api/notifications?page=${page}&perPage=${perPage}`
  if (type) {
    url += `&type=${type}`
  }
  return http.get<PaginatedNotifications>(url, options)
}

export const markNotificationAsRead = async (id: number): Promise<Result> => {
  return await http.post(`/api/notifications/read/${id}`)
}

export const markAllAsRead = async (): Promise<Result> => {
  return await http.post("/api/notifications/read-all")
}

export const purgeReadNotifications = async (): Promise<Result<{purgedCount: number}>> => {
  return await http.post("/api/notifications/purge-read")
}