// ModIndicator converted to Tailwind

import React from "react"
import { heroiconsShieldcheck as IconShield } from "@fider/icons.generated"
import { useFider } from "@fider/hooks"
import { Icon } from "../common"
import { UserRole } from "@fider/models"
import { useUnreadCounts } from "@fider/contexts/UnreadCountsContext"
import { classSet } from "@fider/services"

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
    <a href="/admin/reports" className="relative inline-flex items-center cursor-pointer group">
      <Icon sprite={IconShield} className="h-6 text-muted group-hover:text-foreground" />
      {pendingCount > 0 && (
        <div className={classSet({
          "absolute -top-1.5 flex justify-center items-center bg-primary text-white text-[10px] font-bold min-w-4 h-4 px-1 rounded-badge": true,
          "-right-2": !isOverMaxCount,
          "-right-4": isOverMaxCount,
        })}>
          {displayCount}
        </div>
      )}
    </a>
  )
}
