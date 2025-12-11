import React, { createContext, useContext, useState, useEffect, useCallback, useRef, ReactNode } from "react"
import { UserStatus, UserAvatarType, UserRole } from "@fider/models"
import { actions, Fider, userPermissions } from "@fider/services"
import { useUserStanding } from "@fider/contexts/UserStandingContext"

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
  const globalStanding = useUserStanding()
  const globalStandingRef = useRef(globalStanding)
  
  useEffect(() => {
    globalStandingRef.current = globalStanding
  }, [globalStanding])

  const loadStats = useCallback(async () => {
    const result = await actions.getUserProfileStats(userId)
    if (result.ok) {
      setStats(result.data)
    } else {
      setError("Failed to load profile stats")
    }
  }, [userId])

  const refreshStanding = useCallback(async () => {
    const result = await actions.getUserProfileStanding(userId)
    if (result.ok) {
      setStanding(result.data)
      if (isViewingOwnProfile) {
        globalStandingRef.current.setStandingData(result.data.warnings, result.data.mutes)
      }
    }
  }, [userId, isViewingOwnProfile])

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

  const initialLoadDone = useRef(false)
  
  useEffect(() => {
    if (initialLoadDone.current) {
      return
    }
    initialLoadDone.current = true
    
    const init = async () => {
      setIsLoading(true)
      await loadStats()
      const result = await actions.getUserProfileStanding(userId)
      if (result.ok) {
        setStanding(result.data)
        if (isViewingOwnProfile) {
          globalStandingRef.current.setStandingData(result.data.warnings, result.data.mutes)
        }
      }
      setIsLoading(false)
    }
    init()
  }, [userId, loadStats, isViewingOwnProfile])

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
    canModerate: user ? userPermissions.canModerate(user) : false,
    canBlock: user ? userPermissions.canBlock(user) : false,
    canDeleteModeration: user ? userPermissions.canDeleteModeration(user) : false,
    canEditName: user ? userPermissions.canEditName(user) : false,
    canEditAvatar: user ? userPermissions.canEditAvatar(user) : false,
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

