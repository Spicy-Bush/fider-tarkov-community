export interface Tenant {
  id: number
  name: string
  cname: string
  subdomain: string
  locale: string
  invitation: string
  welcomeMessage: string
  status: TenantStatus
  isPrivate: boolean
  logoBlobKey: string
  isEmailAuthAllowed: boolean
  profanityWords: string
  generalSettings?: {
    titleLengthMin: number
    titleLengthMax: number
    descriptionLengthMin: number
    descriptionLengthMax: number
    maxImagesPerPost: number
    maxImagesPerComment: number
    postLimits: Record<string, { count: number; hours: number }>
    commentLimits: Record<string, { count: number; hours: number }>
    postingDisabledFor: string[]
    commentingDisabledFor: string[]
    postingGloballyDisabled: boolean
    commentingGloballyDisabled: boolean
  }
  messageBanner: string
}

export enum TenantStatus {
  Active = 1,
  Pending = 2,
  Locked = 3,
  Disabled = 4,
}

export enum VisualRole {
  None = "",
  Visitor = "Visitor",
  Helper = "Helper",
  Administrator = "Administrator",
  Moderator = "Moderator",
  BSGCrew = "BSGCrew",
  Developer = "Developer",
  Sherpa = "Sherpa",
  TCStaff = "TCStaff", 
  Emissary = "Emissary"
}

export interface User {
  id: number
  name: string
  email?: string
  role: UserRole
  visualRole?: VisualRole
  status: UserStatus
  avatarURL: string
  providers?: Array<{
    name: string
    uid: string
  }>
}

export interface UserNames {
  id: number
  name: string
}

export enum UserAvatarType {
  Letter = "letter",
  Gravatar = "gravatar",
  Custom = "custom",
}

export enum UserStatus {
  Active = "active",
  Deleted = "deleted",
  Blocked = "blocked",
}

export enum UserRole {
  Visitor = "visitor",
  Collaborator = "collaborator",
  Administrator = "administrator",
  Moderator = "moderator",
  Helper = "helper",
}

export const isHelper = (role: UserRole): boolean => {
  return role === UserRole.Helper || role === UserRole.Moderator || role === UserRole.Administrator
}

export const isCollaborator = (role: UserRole): boolean => {
  return role === UserRole.Collaborator || role === UserRole.Administrator
}

export const isModerator = (role: UserRole): boolean => {
  return role === UserRole.Moderator || role === UserRole.Administrator
}

export const isAdministrator = (role: UserRole): boolean => {
  return role === UserRole.Administrator
}

export interface CurrentUser {
  id: number
  name: string
  email: string
  avatarType: UserAvatarType
  avatarBlobKey: string
  avatarURL: string
  role: UserRole
  visualRole?: VisualRole
  status: UserStatus
  isAdministrator: boolean
  isCollaborator: boolean
  isModerator: boolean
  isHelper: boolean
  hasWarning: boolean
  isMuted: boolean
}

export const getVisualRoleName = (visualRole?: VisualRole): string => {
  if (visualRole === undefined || visualRole === VisualRole.None) return "";
  return visualRole;
}