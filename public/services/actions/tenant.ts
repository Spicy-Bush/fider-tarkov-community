import { http, Result } from "@fider/services/http"
import { User, OAuthConfig, ImageUpload, PermissionAssignment, ResponsePermissionAssignment, SavedRolePermissions } from "@fider/models"
import { UserRole, VisualRole } from "@fider/models/identity"
import { EmailVerificationKind } from "@fider/models/settings"

export interface CheckAvailabilityResponse {
  message: string
}

export interface CreateTenantRequest {
  legalAgreement: boolean
  tenantName: string
  subdomain?: string
  name?: string
  token?: string
  email?: string
}

export interface CreateTenantResponse {
  token?: string
}

export const createTenant = async (request: CreateTenantRequest): Promise<Result<CreateTenantResponse>> => {
  return await http.post<CreateTenantResponse>("/api/tenants", request)
}

export interface UpdateTenantSettingsRequest {
  logo?: ImageUpload
  title: string
  invitation: string
  welcomeMessage: string
  cname: string
  locale: string
}

export const updateGeneralSettings = async (data: { settings: any }): Promise<Result> => {
  return await http.post("/api/admin/settings/content-settings", data)
}

export const updateTenantSettings = async (request: UpdateTenantSettingsRequest): Promise<Result> => {
  return await http.post("/api/admin/settings/general", request)
}

export const updateTenantMessageBanner = async (messageBanner: string): Promise<Result> => {
  return await http.post("/api/admin/settings/message-banner", {
    messageBanner,
  })
}

export const updateTenantAdvancedSettings = async (customCSS: string): Promise<Result> => {
  return await http.post("/api/admin/settings/advanced", {
    customCSS,
  })
}

export async function updateProfanityWords(profanityWords: string) {
  return await http.post("/api/admin/settings/profanity", {
    profanityWords,
  })
}

export const updateTenantPrivacy = async (isPrivate: boolean): Promise<Result> => {
  return await http.post("/api/admin/settings/privacy", {
    isPrivate,
  })
}

export const updateTenantEmailAuthAllowed = async (isEmailAuthAllowed: boolean): Promise<Result> => {
  return await http.post("/api/admin/settings/emailauth", {
    isEmailAuthAllowed,
  })
}

export const checkAvailability = async (subdomain: string): Promise<Result<CheckAvailabilityResponse>> => {
  return await http.get<CheckAvailabilityResponse>(`/api/tenants/${subdomain}/availability`)
}

export const signIn = async (email: string): Promise<Result> => {
  return await http.post("/api/signin", {
    email,
  })
}

export const completeProfile = async (kind: EmailVerificationKind, key: string, name: string): Promise<Result> => {
  return await http.post("/api/signin/complete", {
    kind,
    key,
    name,
  })
}

export type ChangedUserRole = Pick<User, "id" | "role" | "permissions" | "visualRole" | "visualRoleOverride">
export type ChangedVisualRole = Pick<User, "id" | "visualRole" | "visualRoleOverride">

export const changeUserRole = async (userID: number, role: UserRole): Promise<Result<ChangedUserRole>> => {
  return await http.post<ChangedUserRole>(`/api/admin/roles/${role}/users`, {
    userID,
  })
}

export const changeUserVisualRole = async (userID: number, role: VisualRole): Promise<Result<ChangedVisualRole>> => {
  const roleNumbers: Record<VisualRole, number> = {
    "": 0,
    Visitor: 1,
    Helper: 2,
    Administrator: 3,
    Moderator: 4,
    BSGCrew: 5,
    Developer: 6,
    Sherpa: 7,
    TCStaff: 8,
    Emissary: 9,
  }

  return await http.post<ChangedVisualRole>(`/api/admin/visualroles/${roleNumbers[role]}/users`, { userID })
}

export const blockUser = async (userID: number): Promise<Result> => {
  return await http.put(`/api/admin/users/${userID}/block`)
}

export const unblockUser = async (userID: number): Promise<Result> => {
  return await http.delete(`/api/admin/users/${userID}/block`)
}

export const getOAuthConfig = async (provider: string): Promise<Result<OAuthConfig>> => {
  return await http.get<OAuthConfig>(`/api/admin/oauth/${provider}`)
}

export interface CreateEditOAuthConfigRequest {
  provider: string
  status: number
  displayName: string
  clientID: string
  clientSecret: string
  authorizeURL: string
  tokenURL: string
  scope: string
  profileURL: string
  jsonUserIDPath: string
  jsonUserNamePath: string
  jsonUserEmailPath: string
  logo?: ImageUpload
  isTrusted: boolean
}

export const saveOAuthConfig = async (request: CreateEditOAuthConfigRequest): Promise<Result> => {
  return await http.post("/api/admin/oauth", request)
}

export interface NavigationLinkInput {
  title: string
  url: string
  displayOrder: number
  location: string
}

export const saveNavigationLinks = async (links: NavigationLinkInput[]): Promise<Result> => {
  return await http.post("/api/admin/navigation", { links })
}

export const updateRolePermissions = async (submissionId: string, changes: PermissionAssignment[], responseChanges: ResponsePermissionAssignment[]): Promise<Result<SavedRolePermissions>> => {
  return await http.put<SavedRolePermissions>("/api/admin/permissions", { submissionId, changes, responseChanges })
}
