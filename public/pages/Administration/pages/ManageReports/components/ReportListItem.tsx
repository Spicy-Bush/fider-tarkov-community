import React, { useCallback, useMemo, memo } from "react"
import { Icon, Avatar, Moment } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { Fider, classSet } from "@fider/services"
import { heroiconsEye as IconEye } from "@fider/icons.generated"
import { Report, getReportTypeLabel, ViewerInfo } from "@fider/models"

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
    "c-report-list-item": true,
    "c-report-list-item--selected": isSelected,
    "c-report-list-item--pending": report.status === "pending",
    "c-report-list-item--in-review": report.status === "in_review",
  })

  const otherViewers = useMemo(() => 
    viewers.filter((v) => v.userId !== Fider.session.user.id),
    [viewers]
  )

  return (
    <div className={className} onClick={handleClick}>
      <div className="c-report-list-item__header">
        <span className="c-report-list-item__type">
          {getReportTypeLabel(report.reportedType)}
        </span>
        <HStack spacing={1}>
          <span
            className="c-report-list-item__viewers"
            data-tooltip={otherViewers.length > 0 ? otherViewers.map((v) => v.userName).join(", ") : undefined}
            style={{ visibility: otherViewers.length > 0 ? "visible" : "hidden" }}
          >
            <Icon sprite={IconEye} className="h-3" />
            <span>{otherViewers.length || 1}</span>
          </span>
          <span className="c-report-list-item__status">
            {report.status.replace("_", " ")}
          </span>
        </HStack>
      </div>
      <div className="c-report-list-item__reason">{report.reason}</div>
      <div className="c-report-list-item__meta">
        <Avatar user={report.reporter} clickable={false} />
        <span>{report.reporter.name}</span>
        <Moment locale={Fider.currentLocale} date={report.createdAt} />
      </div>
    </div>
  )
})

ReportListItem.displayName = "ReportListItem"
