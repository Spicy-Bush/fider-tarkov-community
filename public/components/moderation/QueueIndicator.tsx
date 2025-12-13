// QueueIndicator converted to Tailwind

import React from "react"
import { heroiconsInbox as IconInbox } from "@fider/icons.generated"
import { useFider } from "@fider/hooks"
import { Icon } from "../common"
import { UserRole } from "@fider/models"
import { useUnreadCounts } from "@fider/contexts/UnreadCountsContext"
import { classSet } from "@fider/services"

export const QueueIndicator = () => {
  const fider = useFider()
  const { counts } = useUnreadCounts()
  const queueCount = counts.queueCount

  const canViewQueue = fider.session.isAuthenticated && 
    fider.session.user.role !== UserRole.Visitor

  if (!canViewQueue) {
    return null
  }

  const isOverMaxCount = queueCount > 99
  const displayCount = isOverMaxCount ? "99+" : queueCount.toString()

  return (
    <a href="/admin/queue" className="relative inline-flex items-center cursor-pointer group">
      <Icon sprite={IconInbox} className="h-6 text-muted group-hover:text-foreground" />
      {queueCount > 0 && (
        <div className={classSet({
          "absolute -top-1.5 flex justify-center items-center bg-success text-white text-[10px] font-bold min-w-4 h-4 px-1 rounded-badge": true,
          "-right-2": !isOverMaxCount,
          "-right-4": isOverMaxCount,
        })}>
          {displayCount}
        </div>
      )}
    </a>
  )
}

