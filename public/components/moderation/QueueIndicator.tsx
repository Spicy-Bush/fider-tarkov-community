import "./QueueIndicator.scss"
import React from "react"
import { heroiconsInbox as IconInbox } from "@fider/icons.generated"
import { useFider } from "@fider/hooks"
import { Icon } from "../common"
import { UserRole } from "@fider/models"
import { useUnreadCounts } from "@fider/contexts/UnreadCountsContext"

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
    <a href="/admin/queue" className="c-queue-indicator">
      <Icon sprite={IconInbox} className="h-6 text-gray-500" />
      {queueCount > 0 && (
        <div className={`c-queue-indicator-counter ${isOverMaxCount ? "is-max-count" : ""}`}>
          {displayCount}
        </div>
      )}
    </a>
  )
}

