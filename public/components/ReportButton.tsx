import React from "react"
import { Icon } from "@fider/components"
import { classSet } from "@fider/services"
import { useFider } from "@fider/hooks"
import { heroiconsFlag as IconFlag } from "@fider/icons.generated"
import "./ReportButton.scss"

interface ReportButtonProps {
  reportedUserId: number
  size?: "small" | "medium"
  hasReported: boolean
  dailyLimitReached: boolean
  onReport: () => void
}

export const ReportButton: React.FC<ReportButtonProps> = ({
  reportedUserId,
  size = "small",
  hasReported,
  dailyLimitReached,
  onReport,
}) => {
  const fider = useFider()

  if (!fider.session.isAuthenticated || fider.session.user.id === reportedUserId) {
    return null
  }

  if (hasReported) {
    return null
  }

  const iconSize = size === "small" ? 16 : 20

  const handleClick = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (dailyLimitReached) {
      return
    }
    onReport()
  }

  const className = classSet({
    "c-report-flag": true,
    "c-report-flag--disabled": dailyLimitReached,
  })

  const button = (
    <button
      type="button"
      onClick={handleClick}
      className={className}
      disabled={dailyLimitReached}
    >
      <Icon sprite={IconFlag} width={iconSize.toString()} height={iconSize.toString()} />
    </button>
  )

  if (dailyLimitReached) {
    return (
      <span data-tooltip="You have reached your daily report limit">
        {button}
      </span>
    )
  }

  return button
}
