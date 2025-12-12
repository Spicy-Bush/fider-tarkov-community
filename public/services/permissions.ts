import { CurrentUser, UserRole } from "@fider/models"
import { Fider } from "./fider"
import { TIME } from "./constants"

type RoleValue = UserRole | number

const normalizeRole = (role: RoleValue): UserRole => {
  if (typeof role === "string") return role
  const roleMap: Record<number, UserRole> = {
    1: UserRole.Visitor,
    2: UserRole.Collaborator, 
    3: UserRole.Administrator,
    4: UserRole.Moderator,
    5: UserRole.Helper,
  }
  return roleMap[role] || UserRole.Visitor
}

const hasRole = (role: RoleValue, expected: UserRole): boolean => {
  return normalizeRole(role) === expected
}

const isStaff = (role: RoleValue): boolean => {
  const normalized = normalizeRole(role)
  return (
    normalized === UserRole.Administrator ||
    normalized === UserRole.Collaborator ||
    normalized === UserRole.Moderator ||
    normalized === UserRole.Helper
  )
}

const isAdmin = (role: RoleValue): boolean => hasRole(role, UserRole.Administrator)
const isCollaborator = (role: RoleValue): boolean => hasRole(role, UserRole.Collaborator) || isAdmin(role)
const isModerator = (role: RoleValue): boolean => hasRole(role, UserRole.Moderator) || isAdmin(role)
const isHelper = (role: RoleValue): boolean => hasRole(role, UserRole.Helper) || isModerator(role)

const getCurrentUser = (): CurrentUser | undefined => {
  if (!Fider.session.isAuthenticated) return undefined
  return Fider.session.user
}

interface UserLike {
  id: number
  role: RoleValue
}

interface PostLike {
  user: UserLike
  createdAt: string
  status?: string
}

interface CommentLike {
  user: UserLike
  createdAt: string
}

const timeAgo = (date: string | Date): number => {
  const d = date instanceof Date ? date : new Date(date)
  return (new Date().getTime() - d.getTime()) / 1000
}

export const postPermissions = {
  canEdit: (post: PostLike, user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    if (currentUser.isCollaborator || currentUser.isAdministrator) return true
    if (currentUser.isModerator) {
      return hasRole(post.user.role, UserRole.Visitor)
    }
    return currentUser.id === post.user.id && timeAgo(post.createdAt) <= TIME.ONE_HOUR_SECONDS
  },

  canDelete: (post: PostLike, user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    if (currentUser.isCollaborator || currentUser.isAdministrator) return true
    if (currentUser.isModerator) {
      return hasRole(post.user.role, UserRole.Visitor)
    }
    return false
  },

  canRespond: (user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    return currentUser.isCollaborator || currentUser.isModerator || currentUser.isAdministrator
  },

  canLock: (user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    return currentUser.isCollaborator || currentUser.isAdministrator
  },

  canArchive: (user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    return currentUser.isCollaborator || currentUser.isAdministrator
  },

  canTag: (user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    return currentUser.isCollaborator || currentUser.isModerator || currentUser.isAdministrator || currentUser.isHelper
  },
}

export const commentPermissions = {
  canEdit: (comment: CommentLike, user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    if (currentUser.isCollaborator || currentUser.isAdministrator) return true
    if (currentUser.isModerator) {
      return hasRole(comment.user.role, UserRole.Visitor)
    }
    return currentUser.id === comment.user.id && timeAgo(comment.createdAt) <= TIME.ONE_HOUR_SECONDS
  },

  canDelete: (comment: CommentLike, user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    if (currentUser.isCollaborator || currentUser.isAdministrator) return true
    if (currentUser.isModerator) {
      return hasRole(comment.user.role, UserRole.Visitor)
    }
    return currentUser.id === comment.user.id
  },
}

export const userPermissions = {
  canModerate: (targetUser: UserLike, user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    if (hasRole(currentUser.role, UserRole.Visitor)) return false
    if (currentUser.id === targetUser.id) return false
    if (isAdmin(targetUser.role)) return false
    
    if (hasRole(currentUser.role, UserRole.Moderator)) {
      return hasRole(targetUser.role, UserRole.Visitor)
    }
    if (hasRole(currentUser.role, UserRole.Collaborator)) {
      return hasRole(targetUser.role, UserRole.Visitor) || hasRole(targetUser.role, UserRole.Moderator)
    }
    return isAdmin(currentUser.role)
  },

  canBlock: (targetUser: UserLike, user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    if (!isAdmin(currentUser.role) && !hasRole(currentUser.role, UserRole.Collaborator)) return false
    if (currentUser.id === targetUser.id) return false
    if (isAdmin(targetUser.role)) return false
    if (hasRole(currentUser.role, UserRole.Collaborator)) {
      return hasRole(targetUser.role, UserRole.Visitor)
    }
    return true
  },

  canWarn: (targetUser: UserLike, user?: CurrentUser): boolean => {
    return userPermissions.canModerate(targetUser, user)
  },

  canMute: (targetUser: UserLike, user?: CurrentUser): boolean => {
    return userPermissions.canModerate(targetUser, user)
  },

  canDeleteModeration: (targetUser: UserLike, user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    if (!isAdmin(currentUser.role) && !hasRole(currentUser.role, UserRole.Collaborator)) return false
    if (currentUser.id === targetUser.id) return false
    if (isAdmin(targetUser.role)) return false
    if (hasRole(currentUser.role, UserRole.Collaborator)) {
      return hasRole(targetUser.role, UserRole.Visitor)
    }
    return true
  },

  canEditName: (targetUser: UserLike, user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    if (currentUser.id === targetUser.id) return true
    return isAdmin(currentUser.role) || 
           hasRole(currentUser.role, UserRole.Moderator) || 
           hasRole(currentUser.role, UserRole.Collaborator)
  },

  canEditAvatar: (targetUser: UserLike, user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    if (currentUser.id === targetUser.id) return true
    return isAdmin(currentUser.role) || 
           hasRole(currentUser.role, UserRole.Moderator) || 
           hasRole(currentUser.role, UserRole.Collaborator)
  },

  canChangeRole: (targetUser: UserLike, user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    if (!isAdmin(currentUser.role) && !hasRole(currentUser.role, UserRole.Collaborator)) return false
    if (currentUser.id === targetUser.id) return false
    return true
  },
}

export const reportPermissions = {
  canAssign: (user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    return currentUser.isCollaborator || currentUser.isModerator || currentUser.isAdministrator
  },

  canResolve: (user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    return currentUser.isCollaborator || currentUser.isModerator || currentUser.isAdministrator
  },

  canDismiss: (user?: CurrentUser): boolean => {
    return reportPermissions.canResolve(user)
  },
}

export const adminPermissions = {
  canAccessAdmin: (user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    return isStaff(currentUser.role)
  },

  canManageSettings: (user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    return currentUser.isAdministrator
  },

  canManageMembers: (user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    return currentUser.isCollaborator || currentUser.isAdministrator
  },

  canManageTags: (user?: CurrentUser): boolean => {
    const currentUser = user ?? getCurrentUser()
    if (!currentUser) return false
    return currentUser.isCollaborator || currentUser.isAdministrator
  },
}

export const permissions = {
  post: postPermissions,
  comment: commentPermissions,
  user: userPermissions,
  report: reportPermissions,
  admin: adminPermissions,
  normalizeRole,
  hasRole,
  isStaff,
  isAdmin,
  isCollaborator,
  isModerator,
  isHelper,
}

