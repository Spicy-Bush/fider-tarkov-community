import React, { useCallback, useMemo, memo } from "react"
import { Icon, Avatar, Moment } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { Fider, classSet } from "@fider/services"
import { heroiconsEye as IconEye } from "@fider/icons.generated"
import { Report, getReportTypeLabel, ViewerInfo, ReportStatus } from "@fider/models"

const getStatusClasses = (status: ReportStatus): string => {
  switch (status) {
    case "resolved":
      return "bg-success-light text-success"
    case "dismissed":
      return "bg-surface-alt text-muted"
    case "in_review":
      return "bg-info-light text-primary"
    case "pending":
    default:
      return "bg-warning-light text-warning"
  }
}

export interface ReportListItemProps {
  report: Report
  isSelected: boolean
  onClick: (report: Report) => void
  viewers: ViewerInfo[]
}

export const ReportListItem: React.FC<ReportListItemProps> = memo(({
  report,
  isSelected,
  onClick,
  viewers,
}) => {
  const handleClick = useCallback(() => {
    onClick(report)
  }, [onClick, report])

  const className = classSet({
    "p-3 cursor-pointer transition-colors border-l-[3px] border-l-transparent hover:bg-surface-alt": true,
    "bg-info-light border-l-primary hover:bg-info-light": isSelected,
    "border-l-warning": report.status === "pending" && !isSelected,
    "border-l-info": report.status === "in_review" && !isSelected,
  })

  const otherViewers = useMemo(() => 
    viewers.filter((v) => v.userId !== Fider.session.user.id),
    [viewers]
  )

  return (
    <div className={className} onClick={handleClick}>
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs font-semibold uppercase text-muted">
          {getReportTypeLabel(report.reportedType)}
        </span>
        <HStack spacing={1}>
          {otherViewers.length > 0 && (
            <span
              className="inline-flex items-center gap-0.5 text-xs text-primary bg-info-medium px-1.5 py-0.5 rounded cursor-help tooltip-left"
              data-tooltip={otherViewers.map((v) => v.userName).join(", ")}
            >
              <Icon sprite={IconEye} className="h-3" />
              <span>{otherViewers.length}</span>
            </span>
          )}
          <span className={classSet({
              "text-xs px-1.5 py-0.5 rounded capitalize": true,
              [getStatusClasses(report.status)]: true,
            })}>
            {report.status.replace("_", " ")}
          </span>
        </HStack>
      </div>
      <div className="text-sm text-foreground font-medium mb-1 whitespace-nowrap overflow-hidden text-ellipsis">{report.reason}</div>
      <div className="flex items-center gap-1 text-xs text-muted [&_.c-avatar]:w-5 [&_.c-avatar]:h-5">
        <Avatar user={report.reporter} clickable={false} />
        <span>{report.reporter.name}</span>
        <Moment locale={Fider.currentLocale} date={report.createdAt} />
      </div>
    </div>
  )
})

ReportListItem.displayName = "ReportListItem"
