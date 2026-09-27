import { http, Result } from "@fider/services"
import { Notification } from "@fider/models"

interface PaginatedNotifications {
  notifications: Notification[]
  total: number
  page: number
  perPage: number
}

export interface UnreadCountsResponse {
  total: number
  pendingReports?: number
  queueCount?: number
}

export const getUnreadCounts = async (): Promise<Result<UnreadCountsResponse>> => {
  return http.get<UnreadCountsResponse>("/api/notifications/unread/total")
}

export const getNotifications = async (
  page: number = 1, 
  perPage: number = 10, 
  type?: string
): Promise<Result<PaginatedNotifications>> => {
  let url = `/api/notifications?page=${page}&perPage=${perPage}`
  if (type) {
    url += `&type=${type}`
  }
  return http.get<PaginatedNotifications>(url)
}

export const markAllAsRead = async (): Promise<Result> => {
  return await http.post("/api/notifications/read-all")
}

export const purgeReadNotifications = async (): Promise<Result<{purgedCount: number}>> => {
  return await http.post("/api/notifications/purge-read")
}