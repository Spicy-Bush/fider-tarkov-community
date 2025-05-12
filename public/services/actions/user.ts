import { http, Result } from "@fider/services/http"
import { UserSettings, UserAvatarType, ImageUpload } from "@fider/models"
import { Fider } from "@fider/services"

interface UserProfileStats {
  posts: number
  comments: number
  votes: number
}

interface UserProfileStanding {
  warnings: Array<{
    id: number
    reason: string
    createdAt: string
    expiresAt?: string
  }>
  mutes: Array<{
    id: number
    reason: string
    createdAt: string
    expiresAt?: string
  }>
}

interface UserProfileContent {
  posts: Array<{
    id: number
    title: string
    createdAt: string
  }>
  comments: Array<{
    id: number
    content: string
    postId: number
    postTitle: string
    createdAt: string
  }>
}

interface ModerateUserRequest {
  reason: string
  duration?: number
  expiresAt?: string
}

interface MuteUserRequest {
  reason: string
  duration: string
}

interface WarnUserRequest {
  reason: string
  duration: string
}

export const updateUserSettings = async (data: {
  settings: UserSettings
}): Promise<Result<void>> => {
  return await http.post("/_api/user/settings", data)
}

export const updateUserAvatar = async (data: {
  avatarType: UserAvatarType
  avatar?: ImageUpload
}, userID?: number): Promise<Result<void>> => {
  if (userID && userID !== Fider.session.user?.id) {
    return await http.post(`/_api/users/${userID}/avatar`, data)
  }
  return await http.post("/_api/user/avatar", data)
}

export const updateOtherUserAvatar = async (userID: number, data: {
  avatarType: UserAvatarType
  avatar?: ImageUpload
}): Promise<Result<void>> => {
  return await updateUserAvatar(data, userID)
}

export const updateUserName = async (data: {
  name: string
}, userID?: number): Promise<Result<void>> => {
  if (userID && userID !== Fider.session.user?.id) {
    return await http.post(`/_api/users/${userID}/name`, data)
  }
  return await http.post("/_api/user/name", data)
}

export const updateOtherUserName = async (userID: number, data: {
  name: string
}): Promise<Result<void>> => {
  return await updateUserName(data, userID)
}

export const changeUserEmail = async (email: string): Promise<Result<void>> => {
  return await http.post("/_api/user/change-email", {
    email,
  })
}

export const deleteCurrentAccount = async (): Promise<Result<void>> => {
  return await http.delete("/_api/user")
}

export const regenerateAPIKey = async (): Promise<Result<{ apiKey: string }>> => {
  return await http.post<{ apiKey: string }>("/_api/user/regenerate-apikey")
}

export const getUserProfileStats = async (userID: number): Promise<Result<UserProfileStats>> => {
  return await http.get<UserProfileStats>(`/api/v1/user/profile/${userID}/stats`)
}

export const getUserProfileStanding = async (userID: number): Promise<Result<UserProfileStanding>> => {
  return await http.get<UserProfileStanding>(`/api/v1/user/profile/${userID}/standing`)
}

export const searchUserContent = async (
  userID: number, 
  query: string, 
  options?: {
    contentType?: string,
    voteType?: "up" | "down",
    limit?: number,
    offset?: number,
    sortBy?: string,
    sortOrder?: string
  }
): Promise<Result<UserProfileContent>> => {
  let url = `/api/v1/user/profile/${userID}/content/search?q=${encodeURIComponent(query)}`
  
  if (options) {
    if (options.contentType) {
      url += `&contentType=${encodeURIComponent(options.contentType)}`
    }
    if (options.voteType) {
      url += `&voteType=${options.voteType}`
    }
    if (options.limit) {
      url += `&limit=${options.limit}`
    }
    if (options.offset) {
      url += `&offset=${options.offset}`
    }
    if (options.sortBy) {
      url += `&sortBy=${encodeURIComponent(options.sortBy)}`
    }
    if (options.sortOrder) {
      url += `&sortOrder=${encodeURIComponent(options.sortOrder)}`
    }
  }
  
  return await http.get<UserProfileContent>(url)
}

export const moderateUser = async (userID: number, action: 'mute' | 'warning', data: ModerateUserRequest): Promise<Result<void>> => {
  const apiAction = action === 'warning' ? 'warn' : action;
  const response = await http.post(`/_api/admin/users/${userID}/${apiAction}`, data)
  return response
}

export const muteUser = async (userID: number, data: MuteUserRequest): Promise<Result<void>> => {
  const response = await http.post(`/_api/admin/users/${userID}/mute`, data)
  return response
}

export const warnUser = async (userID: number, data: WarnUserRequest): Promise<Result<void>> => {
  const response = await http.post(`/_api/admin/users/${userID}/warn`, data)
  return response
}

export const deleteWarning = async (userID: number, warningID: number): Promise<Result<void>> => {
  const response = await http.delete(`/_api/admin/users/${userID}/warnings/${warningID}`)
  return response
}

export const deleteMute = async (userID: number, muteID: number): Promise<Result<void>> => {
  const response = await http.delete(`/_api/admin/users/${userID}/mutes/${muteID}`)
  return response
}
