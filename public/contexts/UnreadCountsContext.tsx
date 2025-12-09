import React, { createContext, useContext, useState, useEffect, useCallback, ReactNode } from "react"
import { actions } from "@fider/services"
import { useFider } from "@fider/hooks"

interface UnreadCounts {
  notifications: number
  pendingReports: number
}

interface UnreadCountsContextValue {
  counts: UnreadCounts
  refreshCounts: () => void
  setNotificationCount: (count: number) => void
}

const UnreadCountsContext = createContext<UnreadCountsContextValue | null>(null)

export const UnreadCountsProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const fider = useFider()
  const [counts, setCounts] = useState<UnreadCounts>({ notifications: 0, pendingReports: 0 })

  const refreshCounts = useCallback(() => {
    if (fider.session.isAuthenticated) {
      actions.getUnreadCounts().then((result) => {
        if (result.ok) {
          setCounts({
            notifications: result.data.total || 0,
            pendingReports: result.data.pendingReports || 0,
          })
        }
      })
    }
  }, [fider.session.isAuthenticated])

  const setNotificationCount = useCallback((count: number) => {
    setCounts((prev) => ({ ...prev, notifications: count }))
  }, [])

  useEffect(() => {
    refreshCounts()
  }, [refreshCounts])

  return (
    <UnreadCountsContext.Provider
      value={{
        counts,
        refreshCounts,
        setNotificationCount,
      }}
    >
      {children}
    </UnreadCountsContext.Provider>
  )
}

export const useUnreadCounts = (): UnreadCountsContextValue => {
  const context = useContext(UnreadCountsContext)
  if (!context) {
    throw new Error("useUnreadCounts must be used within UnreadCountsProvider")
  }
  return context
}

