import "./NotificationIndicator.scss"
import React from "react"
import { heroiconsBell as IconBell } from "@fider/icons.generated"
import { Icon } from "../common"
import { useUnreadCounts } from "@fider/contexts/UnreadCountsContext"

export const NotificationIndicator = () => {
  const { counts } = useUnreadCounts()
  const unreadNotifications = counts.notifications

  const isOverMaxCount = unreadNotifications > 99
  const displayCount = isOverMaxCount ? "99+" : unreadNotifications.toString()

  return (
    <a href="/notifications" className="c-notification-indicator">
      <Icon sprite={IconBell} className="h-6 text-gray-500" />
      {unreadNotifications > 0 && (
        <div className={`c-notification-indicator-counter ${isOverMaxCount ? "is-max-count" : ""}`}>
          {displayCount}
        </div>
      )}
    </a>
  )
}
