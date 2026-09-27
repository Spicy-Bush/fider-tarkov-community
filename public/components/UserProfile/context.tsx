import React, { createContext, useContext, useState, useEffect, useCallback, useRef, ReactNode } from "react"
import { UserStatus, UserAvatarType, UserRole, VisualRole, UserPermissions, UserProfileStanding } from "@fider/models"
import { actions } from "@fider/services"
import { useFider } from "@fider/hooks"
import { useUserStanding } from "@fider/contexts/UserStandingContext"
import { RequestError } from "@fider/services/http"

export type ProfileTab = "search" | "standing" | "settings"

export interface UserData {
  id: number
  name: string
  role: UserRole | number
  visualRole?: VisualRole | string
  avatarURL: string
  status: UserStatus | number
  permissions: UserPermissions
  avatarType?: UserAvatarType
}

export interface UserProfileStats {
  posts: number
  comments: number
  votes: number
}

export type { UserProfileStanding } from "@fider/models"

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
  refreshProfile: () => Promise<void>
  refreshUser: () => void
  updateUserName: (name: string) => void
  updateUserAvatar: (avatarURL: string, avatarType?: UserAvatarType) => void
  updateUserVisualRole: (visualRole: VisualRole | string) => void
  isViewingOwnProfile: boolean
  canModerate: boolean
  canBlock: boolean
  canDeleteModeration: boolean
  canExpireModeration: boolean
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
  const { session } = useFider()
  const isViewingOwnProfile = session.isAuthenticated && session.user.id === userId
  const [otherUser, setOtherUser] = useState<UserData | null>(isViewingOwnProfile ? null : initialUser || null)
  const user = isViewingOwnProfile ? session.user : otherUser
  const [stats, setStats] = useState<UserProfileStats>({ posts: 0, comments: 0, votes: 0 })
  const [otherStanding, setOtherStanding] = useState<UserProfileStanding>({ warnings: [], mutes: [] })
  const [isLoading, setIsLoading] = useState(!initialUser)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTabState] = useState<ProfileTab>("search")

  useEffect(() => {
    if (!isViewingOwnProfile && initialUser) {
      setOtherUser((previous) => {
        if (!previous) return initialUser

        return {
          ...previous,
          role: initialUser.role,
          visualRole: initialUser.visualRole,
          permissions: initialUser.permissions,
        }
      })
    }
  }, [initialUser?.role, initialUser?.visualRole, initialUser?.permissions, isViewingOwnProfile])

  const globalStanding = useUserStanding()
  const refreshOwnStanding = globalStanding.refetch
  const standing = isViewingOwnProfile ? globalStanding : otherStanding
  const standingRequest = useRef<AbortController>()

  useEffect(() => () => standingRequest.current?.abort(), [userId])

  const loadStats = useCallback(async () => {
    try {
      const result = await actions.getUserProfileStats(userId)
      if (result.ok) {
        setStats(result.data)
      } else {
        setError("Could not load profile stats.")
      }
    } catch (error) {
      if (!(error instanceof RequestError)) {
        throw error
      }

      setError("Could not load profile stats.")
    }
  }, [userId])

  const refreshStanding = useCallback(async () => {
    setError(null)

    if (isViewingOwnProfile) {
      await refreshOwnStanding()
      return
    }

    standingRequest.current?.abort()
    const request = new AbortController()
    standingRequest.current = request

    try {
      const result = await actions.getUserProfileStanding(userId, request.signal)
      if (request.signal.aborted) {
        return
      }

      if (result.ok) {
        setOtherStanding(result.data)
      } else {
        setError("Could not load account standing.")
      }
    } catch (error) {
      if (!request.signal.aborted) {
        if (!(error instanceof RequestError)) {
          throw error
        }

        setError("Could not load account standing.")
      }
    }
  }, [userId, isViewingOwnProfile, refreshOwnStanding])

  const refreshProfile = useCallback(async () => {
    await Promise.all([loadStats(), refreshStanding()])
  }, [loadStats, refreshStanding])

  const refreshUser = useCallback(() => {
    window.location.reload()
  }, [])

  const updateUserName = useCallback(
    (name: string) => {
      if (isViewingOwnProfile) {
        session.updateUserProfile({ name })
      } else {
        setOtherUser((prev) => (prev ? { ...prev, name } : null))
      }

      onUserUpdate?.({ name })
    },
    [isViewingOwnProfile, session, onUserUpdate]
  )

  const updateUserAvatar = useCallback(
    (avatarURL: string, avatarType?: UserAvatarType) => {
      const change = avatarType ? { avatarURL, avatarType } : { avatarURL }

      if (isViewingOwnProfile) {
        session.updateUserProfile(change)
      } else {
        setOtherUser((prev) => (prev ? { ...prev, ...change } : null))
      }

      onUserUpdate?.(change)
    },
    [isViewingOwnProfile, session, onUserUpdate]
  )

  const updateUserVisualRole = useCallback(
    (visualRole: VisualRole | string) => {
      if (isViewingOwnProfile) {
        session.updateUserProfile({ visualRole: visualRole as VisualRole })
      } else {
        setOtherUser((prev) => (prev ? { ...prev, visualRole } : null))
      }

      onUserUpdate?.({ visualRole } as Partial<UserData>)
    },
    [isViewingOwnProfile, session, onUserUpdate]
  )

  const setActiveTab = useCallback(
    (tab: ProfileTab) => {
      setActiveTabState(tab)
      if (!embedded) {
        window.location.hash = tab
      }
    },
    [embedded]
  )

  useEffect(() => {
    let mounted = true
    const init = async () => {
      setIsLoading(true)

      try {
        await refreshProfile()
      } finally {
        if (mounted) {
          setIsLoading(false)
        }
      }
    }

    void init()

    return () => {
      mounted = false
    }
  }, [refreshProfile])

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
    isLoading: isLoading || (isViewingOwnProfile && globalStanding.isLoading),
    error: isViewingOwnProfile ? globalStanding.error || error : error,
    activeTab,
    isEmbedded: embedded,
    compact,
    setActiveTab,
    refreshStanding,
    refreshProfile,
    refreshUser,
    updateUserName,
    updateUserAvatar,
    updateUserVisualRole,
    isViewingOwnProfile,
    canModerate: user ? user.permissions.moderate : false,
    canBlock: user ? user.permissions.block : false,
    canDeleteModeration: user ? user.permissions.deleteModeration : false,
    canExpireModeration: user ? user.permissions.expireModeration : false,
    canEditName: user ? user.permissions.editName : false,
    canEditAvatar: user ? user.permissions.editAvatar : false,
  }

  return <UserProfileContext.Provider value={contextValue}>{children}</UserProfileContext.Provider>
}

export const useUserProfile = (): UserProfileContextType => {
  const context = useContext(UserProfileContext)
  if (!context) {
    throw new Error("useUserProfile must be used within a UserProfileProvider")
  }
  return context
}
