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
    <div className="flex-[0_0_100%] lg:flex-[0_0_500px] lg:min-w-[500px] lg:h-full flex flex-col bg-elevated rounded-panel border border-surface-alt overflow-hidden">
      <div className="flex items-center gap-2 p-3 border-b border-surface-alt bg-tertiary shrink-0 flex-wrap [&_.c-form-field]:mb-0 [&_.c-select]:flex-[0_0_auto] [&_.c-select]:min-w-[120px] [&_.c-select]:max-w-[150px] [&_.c-select_select]:h-9">
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
        <button
          type="button"
          onClick={onRefresh}
          className="w-9 h-9 p-0 shrink-0 flex items-center justify-center rounded-input border border-border bg-elevated text-muted hover:bg-surface-alt hover:text-foreground transition-colors cursor-pointer"
        >
          <Icon sprite={IconRefresh} className="w-4 h-4" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto min-h-0">
        {newReportIds.size > 0 && (
          <button 
            className="flex items-center justify-center gap-2 w-full py-2 px-3 bg-success-light border-none border-b border-success-light text-success font-medium text-sm cursor-pointer hover:bg-success-medium transition-colors [&_svg]:text-success"
            onClick={onRefreshNewReports}
          >
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
        <div className="flex items-center justify-between py-2 px-3 border-t border-surface-alt bg-tertiary shrink-0">
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
