import React, { createContext, useContext, useState, useEffect, useCallback, ReactNode } from "react"
import { UserRole, UserStatus, UserAvatarType } from "@fider/models"
import { actions, Fider } from "@fider/services"

export type ProfileTab = "search" | "standing" | "settings"

export interface UserData {
  id: number
  name: string
  role: UserRole | number
  avatarURL: string
  status: UserStatus | number
  avatarType?: UserAvatarType
}

export interface UserProfileStats {
  posts: number
  comments: number
  votes: number
}

export interface Warning {
  id: number
  reason: string
  createdAt: string
  expiresAt?: string
}

export interface Mute {
  id: number
  reason: string
  createdAt: string
  expiresAt?: string
}

export interface UserProfileStanding {
  warnings: Warning[]
  mutes: Mute[]
}

interface UserProfileState {
  user: UserData | null
  stats: UserProfileStats
  standing: UserProfileStanding
  isLoading: boolean
  error: string | null
  activeTab: ProfileTab
  isEmbedded: boolean
  compact: boolean
}

interface UserProfileContextType extends UserProfileState {
  setActiveTab: (tab: ProfileTab) => void
  refreshStanding: () => Promise<void>
  refreshStats: () => Promise<void>
  refreshUser: () => void
  updateUserName: (name: string) => void
  updateUserAvatar: (avatarURL: string) => void
  isViewingOwnProfile: boolean
  canModerate: boolean
  canBlock: boolean
  canDeleteModeration: boolean
  canEditName: boolean
  canEditAvatar: boolean
}

const UserProfileContext = createContext<UserProfileContextType | null>(null)

interface UserProfileProviderProps {
  userId: number
  user?: UserData
  embedded?: boolean
  compact?: boolean
  onUserUpdate?: (user: Partial<UserData>) => void
  children: ReactNode
}

const isRole = (role: UserRole | number, expected: UserRole): boolean => {
  return role === expected || role === (expected as unknown as number)
}

const canModerateUser = (targetUser: UserData | null): boolean => {
  if (!targetUser) return false
  const currentUser = Fider.session.user
  if (!currentUser) return false
  const currentRole = currentUser.role
  const targetRole = targetUser.role
  if (isRole(currentRole, UserRole.Visitor)) return false
  if (currentUser.id === targetUser.id) return false
  if (isRole(targetRole, UserRole.Administrator)) return false
  if (isRole(currentRole, UserRole.Moderator)) {
    return isRole(targetRole, UserRole.Visitor)
  }
  if (isRole(currentRole, UserRole.Collaborator)) {
    return isRole(targetRole, UserRole.Visitor) || isRole(targetRole, UserRole.Moderator)
  }
  return isRole(currentRole, UserRole.Administrator)
}

const canBlockUser = (targetUser: UserData | null): boolean => {
  if (!targetUser) return false
  const currentUser = Fider.session.user
  if (!currentUser) return false
  if (!isRole(currentUser.role, UserRole.Administrator) && !isRole(currentUser.role, UserRole.Collaborator)) return false
  if (currentUser.id === targetUser.id) return false
  if (isRole(targetUser.role, UserRole.Administrator)) return false
  if (isRole(currentUser.role, UserRole.Collaborator)) {
    return isRole(targetUser.role, UserRole.Visitor)
  }
  return true
}

const canDeleteModerationAction = (targetUser: UserData | null): boolean => {
  if (!targetUser) return false
  const currentUser = Fider.session.user
  if (!currentUser) return false
  if (!isRole(currentUser.role, UserRole.Administrator) && 
      !isRole(currentUser.role, UserRole.Collaborator) && 
      !isRole(currentUser.role, UserRole.Moderator)) return false
  if (currentUser.id === targetUser.id) return false
  if (isRole(targetUser.role, UserRole.Administrator)) return false
  if (isRole(currentUser.role, UserRole.Moderator)) {
    return isRole(targetUser.role, UserRole.Visitor)
  }
  if (isRole(currentUser.role, UserRole.Collaborator)) {
    return isRole(targetUser.role, UserRole.Visitor)
  }
  return true
}

const canEditUserName = (targetUser: UserData | null): boolean => {
  if (!targetUser) return false
  const currentUser = Fider.session.user
  if (!currentUser) return false
  if (currentUser.id === targetUser.id) return true
  return isRole(currentUser.role, UserRole.Administrator) || 
         isRole(currentUser.role, UserRole.Moderator) || 
         isRole(currentUser.role, UserRole.Collaborator)
}

const canEditUserAvatar = (targetUser: UserData | null): boolean => {
  if (!targetUser) return false
  const currentUser = Fider.session.user
  if (!currentUser) return false
  if (currentUser.id === targetUser.id) return true
  return isRole(currentUser.role, UserRole.Administrator) || 
         isRole(currentUser.role, UserRole.Moderator) || 
         isRole(currentUser.role, UserRole.Collaborator)
}

export const UserProfileProvider: React.FC<UserProfileProviderProps> = ({
  userId,
  user: initialUser,
  embedded = false,
  compact = false,
  onUserUpdate,
  children,
}) => {
  const [user, setUser] = useState<UserData | null>(initialUser || null)
  const [stats, setStats] = useState<UserProfileStats>({ posts: 0, comments: 0, votes: 0 })
  const [standing, setStanding] = useState<UserProfileStanding>({ warnings: [], mutes: [] })
  const [isLoading, setIsLoading] = useState(!initialUser)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTabState] = useState<ProfileTab>("search")

  const isViewingOwnProfile = Fider.session.isAuthenticated && Fider.session.user.id === userId

  const loadStats = useCallback(async () => {
    const result = await actions.getUserProfileStats(userId)
    if (result.ok) {
      setStats(result.data)
    } else {
      setError("Failed to load profile stats")
    }
  }, [userId])

  const loadStanding = useCallback(async () => {
    const result = await actions.getUserProfileStanding(userId)
    if (result.ok) {
      setStanding(result.data)
    }
  }, [userId])

  const refreshStanding = useCallback(async () => {
    await loadStanding()
  }, [loadStanding])

  const refreshStats = useCallback(async () => {
    await loadStats()
  }, [loadStats])

  const refreshUser = useCallback(() => {
    window.location.reload()
  }, [])

  const updateUserName = useCallback((name: string) => {
    setUser(prev => prev ? { ...prev, name } : null)
    onUserUpdate?.({ name })
  }, [onUserUpdate])

  const updateUserAvatar = useCallback((avatarURL: string) => {
    setUser(prev => prev ? { ...prev, avatarURL } : null)
    onUserUpdate?.({ avatarURL })
  }, [onUserUpdate])

  const setActiveTab = useCallback((tab: ProfileTab) => {
    setActiveTabState(tab)
    if (!embedded) {
      window.location.hash = tab
    }
  }, [embedded])

  useEffect(() => {
    const init = async () => {
      setIsLoading(true)
      await loadStats()
      await loadStanding()
      setIsLoading(false)
    }
    init()
  }, [userId, loadStats, loadStanding])

  useEffect(() => {
    if (embedded) return
    
    const handleHashChange = () => {
      const hash = window.location.hash.replace("#", "")
      if (hash === "search" || hash === "standing" || hash === "settings") {
        setActiveTabState(hash as ProfileTab)
      }
    }
    handleHashChange()
    window.addEventListener("hashchange", handleHashChange)
    return () => window.removeEventListener("hashchange", handleHashChange)
  }, [embedded])

  const contextValue: UserProfileContextType = {
    user,
    stats,
    standing,
    isLoading,
    error,
    activeTab,
    isEmbedded: embedded,
    compact,
    setActiveTab,
    refreshStanding,
    refreshStats,
    refreshUser,
    updateUserName,
    updateUserAvatar,
    isViewingOwnProfile,
    canModerate: canModerateUser(user),
    canBlock: canBlockUser(user),
    canDeleteModeration: canDeleteModerationAction(user),
    canEditName: canEditUserName(user),
    canEditAvatar: canEditUserAvatar(user),
  }

  return (
    <UserProfileContext.Provider value={contextValue}>
      {children}
    </UserProfileContext.Provider>
  )
}

export const useUserProfile = (): UserProfileContextType => {
  const context = useContext(UserProfileContext)
  if (!context) {
    throw new Error("useUserProfile must be used within a UserProfileProvider")
  }
  return context
}

