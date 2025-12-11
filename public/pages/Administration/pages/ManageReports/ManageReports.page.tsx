import React, { useEffect, useCallback } from "react"
import { PageConfig } from "@fider/components/layouts"
import { useStackNavigation, useRealtimePresence } from "@fider/hooks"
import { useReportsState, useReportsEvents, useReportsActions, ViewingUserType } from "./hooks"
import { ReportsList, ReportsPreview, ResolveModal } from "./components"

import "../ManageReports.page.scss"

export const pageConfig: PageConfig = {
  title: "Reports",
  subtitle: "Review and manage user reports",
  sidebarItem: "reports",
  layoutVariant: "fullWidth",
}

interface HistoryState extends Record<string, unknown> {
  reportId?: number
  userId?: number
}

const ManageReportsPage: React.FC = () => {
  const state = useReportsState()

  const { viewers, viewItem, stopViewing } = useRealtimePresence({
    eventPrefix: "report",
    itemIdField: "reportId",
  })

  const handleStateChange = useCallback(
    (historyState: HistoryState | null) => {
      if (historyState?.userId && historyState?.reportId) {
        const report = state.reports.find((r) => r.id === historyState.reportId)
        if (report) {
          state.setSelectedReport(report)
        }
      } else if (historyState?.reportId) {
        const report = state.reports.find((r) => r.id === historyState.reportId)
        if (report) {
          state.setSelectedReport(report)
          state.setViewingUser(null)
        } else {
          state.setSelectedReport(null)
          state.setViewingUser(null)
        }
      } else {
        state.setSelectedReport(null)
        state.setViewingUser(null)
      }
    },
    [state.reports, state.setSelectedReport, state.setViewingUser]
  )

  const { pushState, isNavigating } = useStackNavigation<HistoryState>({
    onStateChange: handleStateChange,
    urlPath: "/admin/reports",
  })

  const actions = useReportsActions({
    selectedReport: state.selectedReport,
    selectedStatusRef: state.selectedStatusRef,
    resolveAction: state.resolveAction,
    resolutionNote: state.resolutionNote,
    reasons: state.reasons,
    setReports: state.setReports,
    setSelectedReport: state.setSelectedReport,
    setSelectedStatus: state.setSelectedStatus,
    setSelectedType: state.setSelectedType,
    setSelectedReason: state.setSelectedReason,
    setPage: state.setPage,
    setNewReportIds: state.setNewReportIds,
    setViewingUser: state.setViewingUser,
    setProfileKey: state.setProfileKey,
    setShowResolveModal: state.setShowResolveModal,
    setResolveAction: state.setResolveAction,
    setResolutionNote: state.setResolutionNote,
    setError: state.setError,
    loadReports: state.loadReports,
    pushState,
    isNavigating,
  })

  useReportsEvents({
    selectedReportRef: state.selectedReportRef,
    selectedStatusRef: state.selectedStatusRef,
    setReports: state.setReports,
    setSelectedReport: state.setSelectedReport,
    setNewReportIds: state.setNewReportIds,
  })

  useEffect(() => {
    state.loadReports()
  }, [state.loadReports])

  useEffect(() => {
    if (state.selectedReport) {
      state.loadPreviewContent(state.selectedReport.id)
      viewItem(state.selectedReport.id)
    } else {
      stopViewing()
    }
  }, [state.selectedReport?.id])

  useEffect(() => {
    return () => stopViewing()
  }, [stopViewing])

  const handleUserUpdate = useCallback((updates: Partial<ViewingUserType>) => {
    state.setViewingUser((prev) => (prev ? { ...prev, ...updates } : null))
  }, [state.setViewingUser])

  return (
    <div className="c-reports-split-view">
      <ReportsList
        reports={state.reports}
        total={state.total}
        page={state.page}
        perPage={state.perPage}
        isLoading={state.isLoading}
        selectedReport={state.selectedReport}
        selectedStatus={state.selectedStatus}
        selectedType={state.selectedType}
        selectedReason={state.selectedReason}
        newReportIds={state.newReportIds}
        viewers={viewers}
        statusOptions={actions.statusOptions}
        typeOptions={actions.typeOptions}
        reasonOptions={actions.reasonOptions}
        onSelectReport={actions.handleSelectReport}
        onStatusChange={actions.handleStatusChange}
        onTypeChange={actions.handleTypeChange}
        onReasonChange={actions.handleReasonChange}
        onRefresh={state.loadReports}
        onRefreshNewReports={actions.handleRefreshNewReports}
        onPrevPage={actions.handlePrevPage}
        onNextPage={actions.handleNextPage}
      />
      <ReportsPreview
        selectedReport={state.selectedReport}
        viewingUser={state.viewingUser}
        profileKey={state.profileKey}
        previewPost={state.previewPost}
        previewComment={state.previewComment}
        isLoadingPreview={state.isLoadingPreview}
        onDeselectReport={actions.handleDeselectReport}
        onCloseUserProfile={actions.handleCloseUserProfile}
        onAssign={actions.handleAssign}
        onUnassign={actions.handleUnassign}
        onResolveClick={actions.handleResolveClick}
        onViewUser={actions.handleViewUser}
        onUserUpdate={handleUserUpdate}
      />
      <ResolveModal
        isOpen={state.showResolveModal}
        resolveAction={state.resolveAction}
        resolutionNote={state.resolutionNote}
        error={state.error}
        onResolutionNoteChange={state.setResolutionNote}
        onSubmit={actions.handleResolveSubmit}
        onClose={actions.handleCloseResolveModal}
      />
    </div>
  )
}

export default ManageReportsPage

