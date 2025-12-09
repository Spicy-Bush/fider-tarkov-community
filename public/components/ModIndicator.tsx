import "./ModIndicator.scss"
import React, { useEffect, useState } from "react"
import IconShield from "@fider/assets/images/heroicons-shieldcheck.svg"
import NoDataIllustration from "@fider/assets/images/undraw-empty.svg"
import { useIsStaff } from "@fider/hooks"
import { actions, Fider } from "@fider/services"
import { Icon, Moment } from "./common"
import { Dropdown } from "./common/Dropdown"
import { Report, ReportStatus, getReportTypeLabel } from "@fider/models"
import { HStack, VStack } from "./layout"
import { Trans } from "@lingui/react/macro"
import { useUnreadCounts } from "@fider/contexts/UnreadCountsContext"

const ReportSkeleton = () => {
  return (
    <HStack spacing={4} className="px-3 pr-5 py-4">
      <div className="skeleton h-8 w-8 rounded-full"></div>
      <div className="flex-grow">
        <div className="skeleton h-4 w-3/4 mb-2"></div>
        <div className="skeleton h-3 w-1/4"></div>
      </div>
    </HStack>
  )
}

const ReportItem = ({ report }: { report: Report }) => {
  const openReport = () => {
    window.location.href = `/admin/reports?highlight=${report.id}`
  }

  return (
    <HStack spacing={4} className="px-3 pr-5 clickable hover py-3" onClick={openReport}>
      <div className="c-mod-indicator__type-badge">{getReportTypeLabel(report.reportedType)}</div>
      <div className="flex-grow">
        <p className="c-mod-indicator__reason">{report.reason}</p>
        <span className="text-muted text-xs">
          <Trans id="mod.report.by">by</Trans> {report.reporter.name} -{" "}
          <Moment locale={Fider.currentLocale} date={report.createdAt} />
        </span>
      </div>
    </HStack>
  )
}

const ModIcon = ({ pendingCount }: { pendingCount: number }) => {
  const isOverMaxCount = pendingCount > 99
  const displayCount = isOverMaxCount ? "99+" : pendingCount.toString()

  return (
    <span className="c-mod-indicator mr-3">
      <Icon sprite={IconShield} className="h-6 text-gray-500" />
      {pendingCount > 0 && (
        <div className={`c-mod-indicator-counter ${isOverMaxCount ? "is-max-count" : ""}`}>
          {displayCount}
        </div>
      )}
    </span>
  )
}

export const ModIndicator = () => {
  const isStaff = useIsStaff()
  const { counts } = useUnreadCounts()
  const [showingReports, setShowingReports] = useState(false)
  const [loading, setLoading] = useState(false)
  const [reports, setReports] = useState<Report[]>([])
  const pendingCount = counts.pendingReports

  useEffect(() => {
    if (showingReports && isStaff) {
      setLoading(true)
      actions.listReports({ status: "pending" as ReportStatus, perPage: 10 }).then((result) => {
        if (result.ok) {
          setReports(result.data.reports || [])
        }
        setLoading(false)
      })
    }
  }, [showingReports, isStaff])

  if (!isStaff) {
    return null
  }

  return (
    <Dropdown
      wide={true}
      position="left"
      fullscreenSm={true}
      onToggled={(isOpen: boolean) => setShowingReports(isOpen)}
      renderHandle={<ModIcon pendingCount={pendingCount} />}
    >
      <div className="c-mod-indicator__container">
        {showingReports && (
          <>
            <div className="flex justify-between items-center px-4 py-3 border-b">
              <a href="/admin/reports" className="text-medium font-semibold mb-0 text-link hover:underline">
                <Trans id="mod.reports.title">Reports</Trans>
              </a>
            </div>

            <div className="c-mod-indicator__scroll-container">
              {loading ? (
                <VStack spacing={0} className="py-2" divide={false}>
                  {Array(3)
                    .fill(null)
                    .map((_, i) => (
                      <ReportSkeleton key={i} />
                    ))}
                </VStack>
              ) : reports.length > 0 ? (
                <VStack spacing={0} divide={true}>
                  {reports.map((report) => (
                    <ReportItem key={report.id} report={report} />
                  ))}
                </VStack>
              ) : (
                <div className="text-center py-6">
                  <p className="text-display text-center mt-4 px-4">
                    <Trans id="mod.reports.empty">No pending reports</Trans>
                  </p>
                  <Icon sprite={NoDataIllustration} height="100" className="mt-4 mb-2" />
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </Dropdown>
  )
}

