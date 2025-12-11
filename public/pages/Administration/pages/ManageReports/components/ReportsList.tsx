import React from "react"
import { Button, Loader, Select, SelectOption, Icon } from "@fider/components"
import { VStack } from "@fider/components/layout"
import { heroiconsRefresh as IconRefresh } from "@fider/icons.generated"
import { Report, ReportStatus, ReportType, ViewerInfo } from "@fider/models"
import { Trans } from "@lingui/react/macro"
import { ReportListItem } from "./ReportListItem"

interface ReportsListProps {
  reports: Report[]
  total: number
  page: number
  perPage: number
  isLoading: boolean
  selectedReport: Report | null
  selectedStatus: ReportStatus | "active"
  selectedType: ReportType | ""
  selectedReason: string
  newReportIds: Set<number>
  viewers: Map<number, ViewerInfo[]>
  statusOptions: SelectOption[]
  typeOptions: SelectOption[]
  reasonOptions: SelectOption[]
  onSelectReport: (report: Report) => void
  onStatusChange: (option?: SelectOption) => void
  onTypeChange: (option?: SelectOption) => void
  onReasonChange: (option?: SelectOption) => void
  onRefresh: () => void
  onRefreshNewReports: () => void
  onPrevPage: () => void
  onNextPage: () => void
}

export const ReportsList: React.FC<ReportsListProps> = ({
  reports,
  total,
  page,
  perPage,
  isLoading,
  selectedReport,
  selectedStatus,
  selectedType,
  selectedReason,
  newReportIds,
  viewers,
  statusOptions,
  typeOptions,
  reasonOptions,
  onSelectReport,
  onStatusChange,
  onTypeChange,
  onReasonChange,
  onRefresh,
  onRefreshNewReports,
  onPrevPage,
  onNextPage,
}) => {
  const getViewersForReport = (reportId: number): ViewerInfo[] => {
    return viewers.get(reportId) || []
  }

  return (
    <div className="c-reports-split-view__list">
      <div className="c-reports-split-view__filters">
        <Select
          field="status"
          defaultValue={selectedStatus}
          options={statusOptions}
          onChange={onStatusChange}
        />
        <Select
          field="type"
          defaultValue={selectedType}
          options={typeOptions}
          onChange={onTypeChange}
        />
        <Select
          field="reason"
          defaultValue={selectedReason}
          options={reasonOptions}
          onChange={onReasonChange}
        />
        <Button
          variant="secondary"
          size="small"
          onClick={onRefresh}
          className="c-reports-split-view__refresh-btn"
        >
          <Icon sprite={IconRefresh} />
        </Button>
      </div>

      <div className="c-reports-split-view__list-content">
        {newReportIds.size > 0 && (
          <button className="c-reports-new-banner" onClick={onRefreshNewReports}>
            <Icon sprite={IconRefresh} className="h-4" />
            <span>
              {newReportIds.size} new report{newReportIds.size > 1 ? "s" : ""} available
            </span>
          </button>
        )}
        {isLoading ? (
          <div className="py-8 text-center">
            <Loader />
          </div>
        ) : reports.length === 0 ? (
          <div className="py-8 text-center text-muted">
            <Trans id="reports.empty">No reports found.</Trans>
          </div>
        ) : (
          <VStack spacing={0} divide>
            {reports.map((report) => (
              <ReportListItem
                key={report.id}
                report={report}
                isSelected={selectedReport?.id === report.id}
                onClick={onSelectReport}
                viewers={getViewersForReport(report.id)}
              />
            ))}
          </VStack>
        )}
      </div>

      {total > reports.length && (
        <div className="c-reports-split-view__pagination">
          <Button size="small" variant="tertiary" disabled={page === 1} onClick={onPrevPage}>
            Prev
          </Button>
          <span className="text-muted text-sm">Page {page}</span>
          <Button size="small" variant="tertiary" disabled={reports.length < perPage} onClick={onNextPage}>
            Next
          </Button>
        </div>
      )}
    </div>
  )
}

