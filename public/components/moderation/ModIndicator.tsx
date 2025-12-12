import "./ModIndicator.scss"
import React from "react"
import { heroiconsShieldcheck as IconShield } from "@fider/icons.generated"
import { useFider } from "@fider/hooks"
import { Icon } from "../common"
import { UserRole } from "@fider/models"
import { useUnreadCounts } from "@fider/contexts/UnreadCountsContext"

export const ModIndicator = () => {
  const fider = useFider()
  const { counts } = useUnreadCounts()
  const pendingCount = counts.pendingReports

  const canViewReports = fider.session.isAuthenticated && 
    fider.session.user.role !== UserRole.Helper &&
    fider.session.user.role !== UserRole.Visitor

  if (!canViewReports) {
    return null
  }

  const isOverMaxCount = pendingCount > 99
  const displayCount = isOverMaxCount ? "99+" : pendingCount.toString()

  return (
    <a href="/admin/reports" className="c-mod-indicator">
      <Icon sprite={IconShield} className="h-6 text-gray-500" />
      {pendingCount > 0 && (
        <div className={`c-mod-indicator-counter ${isOverMaxCount ? "is-max-count" : ""}`}>
          {displayCount}
        </div>
      )}
    </a>
  )
}
