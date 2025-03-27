import { http, Result } from "@fider/services"
import { Notification } from "@fider/models"

interface PaginatedNotifications {
  notifications: Notification[]
  total: number
  page: number
  perPage: number
}

export const getTotalUnreadNotifications = async (): Promise<Result<number>> => {
  return http.get<{ total: number }>("/_api/notifications/unread/total").then((result) => {
    return {
      ok: result.ok,
      error: result.error,
      data: result.data ? result.data.total : 0,
    }
  })
}

export const getNotifications = async (
  page: number = 1, 
  perPage: number = 10, 
  type?: string
): Promise<Result<PaginatedNotifications>> => {
  let url = `/_api/notifications?page=${page}&perPage=${perPage}`
  if (type) {
    url += `&type=${type}`
  }
  return http.get<PaginatedNotifications>(url)
}

export const markAllAsRead = async (): Promise<Result> => {
  return await http.post("/_api/notifications/read-all")
}

export const purgeReadNotifications = async (): Promise<Result<{purgedCount: number}>> => {
  return await http.post("/_api/notifications/purge-read")
}