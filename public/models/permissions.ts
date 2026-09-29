import type { UserRole } from "./identity"
import type { PostStatusValue } from "./post"

export interface SessionPermissions {
  readSettings: boolean
  manageProfanity: boolean
  createUsers: boolean
  viewDesignSystem: boolean
  manageAPIKeys: boolean
  changeOwnEmail: boolean
  enableEmailNotifications: boolean
  manageSettings: boolean
  manageContentSettings: boolean
  managePages: boolean
  managePageTopics: boolean
  manageNavigation: boolean
  viewPrivateTags: boolean
  manageTags: boolean
  manageMembers: boolean
  manageReports: boolean
  manageReportReasons: boolean
  manageQueue: boolean
  manageArchive: boolean
  manageResponses: boolean
  readResponses: boolean
  manageSponsorship: boolean
  manageWebhooks: boolean
  manageRolePermissions: boolean
  manageAuthentication: boolean
  manageInvitations: boolean
  manageBilling: boolean
  manageFiles: boolean
  exportBackup: boolean
  readProfiles: boolean
  editUserProfiles: boolean
  moderateUsers: boolean
  deleteUserModeration: boolean
  expireUserModeration: boolean
  blockUsers: boolean
  changeUserRoles: boolean
  changeUserVisualRoles: boolean
  readUserEmails: boolean
  bypassContentRestrictions: boolean
  bypassPostingRateLimits: boolean
  tagPostsOutsideWindow: boolean
  createPosts: boolean
  editPosts: boolean
  deletePosts: boolean
  respondToPosts: boolean
  lockPosts: boolean
  moderatePosts: boolean
  tagPosts: boolean
  viewPostVotes: boolean
  exportFeedback: boolean
}

export interface UserPermissions {
  readProfile: boolean
  editName: boolean
  editAvatar: boolean
  block: boolean
  moderate: boolean
  deleteModeration: boolean
  expireModeration: boolean
  changeRole: boolean
  changeVisualRole: boolean
}

export type Permission = Exclude<keyof SessionPermissions, "respondToPosts">

export type RolePermissions = Record<UserRole, Permission[]>

export type PermissionRequirements = Partial<Record<Permission, Permission[]>>

export type RolePermissionLocks = Partial<Record<UserRole, Partial<Record<Permission, string>>>>

export type RoleResponsePermissions = Record<UserRole, PostStatusValue[]>

export interface SavedRolePermissions {
  permissions: RolePermissions
  baseLocks: RolePermissionLocks
  requires: PermissionRequirements
  responses: RoleResponsePermissions
  defaultResponses: RoleResponsePermissions
  responseOptions: PostStatusValue[]
  responseLocks: Partial<Record<UserRole, Partial<Record<PostStatusValue, string>>>>
  blocked?: string
}

export interface PermissionAssignment {
  role: UserRole
  permission: Permission
  granted: boolean
}

export interface ResponsePermissionAssignment {
  role: UserRole
  status: PostStatusValue
  granted: boolean
}
