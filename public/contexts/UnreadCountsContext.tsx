import React, { createContext, useContext, useState, useEffect, useSyncExternalStore, ReactNode } from "react"
import { useFider } from "@fider/hooks/use-fider"
import { createNotificationInbox, NotificationInboxSnapshot } from "@fider/services/notificationInbox"

type NotificationInbox = ReturnType<typeof createNotificationInbox>

interface UnreadCountsContextValue {
  counts: { notifications: number; pendingReports: number; queueCount: number }
  refreshCounts: NotificationInbox["refreshCounts"]
  inbox: NotificationInbox
  snapshot: NotificationInboxSnapshot
}

const UnreadCountsContext = createContext<UnreadCountsContextValue | null>(null)

export const UnreadCountsProvider = ({ children }: { children: ReactNode }) => {
  const { session } = useFider()
  const userID = session.isAuthenticated ? session.user.id : undefined
  return (
    <AccountNotifications key={`${session.tenant.id}:${userID}`} tenantID={session.tenant.id} userID={userID}>
      {children}
    </AccountNotifications>
  )
}

function AccountNotifications({ tenantID, userID, children }: { tenantID: number; userID?: number; children: ReactNode }) {
  const fider = useFider()
  const [inbox] = useState(() => createNotificationInbox(() =>
    fider.session.isAuthenticated && fider.session.tenant.id === tenantID && fider.session.user.id === userID
  ))
  const snapshot = useSyncExternalStore(inbox.subscribe, inbox.getSnapshot, inbox.getSnapshot)
  useEffect(inbox.start, [inbox])

  return (
    <UnreadCountsContext.Provider value={{
      counts: { notifications: snapshot.totals.unread, pendingReports: snapshot.pendingReports, queueCount: snapshot.queueCount },
      refreshCounts: inbox.refreshCounts,
      inbox,
      snapshot,
    }}>
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
