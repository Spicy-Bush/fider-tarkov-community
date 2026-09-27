import { useCallback, useMemo } from "react"
import { Report, ReportStatus, ReportType, ReportReason } from "@fider/models"
import { actions, Fider, Failure } from "@fider/services"
import { SelectOption } from "@fider/components"
import { i18n } from "@lingui/core"
import { ViewingUserType } from "./useReportsState"

interface UseReportsActionsConfig {
  selectedReport: Report | null
  selectedStatusRef: React.MutableRefObject<ReportStatus | "active">
  resolveAction: "resolved" | "dismissed"
  resolutionNote: string
  reasons: ReportReason[]
  updateReport: (report: Report) => void
  removeReport: (reportId: number) => void
  setSelectedReport: React.Dispatch<React.SetStateAction<Report | null>>
  setSelectedStatus: React.Dispatch<React.SetStateAction<ReportStatus | "active">>
  setSelectedType: React.Dispatch<React.SetStateAction<ReportType | "">>
  setSelectedReason: React.Dispatch<React.SetStateAction<string>>
  setPage: React.Dispatch<React.SetStateAction<number>>
  clearNewReports: () => void
  setViewingUser: React.Dispatch<React.SetStateAction<ViewingUserType | null>>
  setProfileKey: React.Dispatch<React.SetStateAction<number>>
  setShowResolveModal: React.Dispatch<React.SetStateAction<boolean>>
  setResolveAction: React.Dispatch<React.SetStateAction<"resolved" | "dismissed">>
  setResolutionNote: React.Dispatch<React.SetStateAction<string>>
  setError: React.Dispatch<React.SetStateAction<Failure | undefined>>
  loadReports: () => Promise<void>
  pushState: (state: Record<string, unknown>) => void
  isNavigating: React.MutableRefObject<boolean>
}

interface UseReportsActionsResult {
  statusOptions: SelectOption[]
  typeOptions: SelectOption[]
  reasonOptions: SelectOption[]
  handleStatusChange: (option?: SelectOption) => void
  handleTypeChange: (option?: SelectOption) => void
  handleReasonChange: (option?: SelectOption) => void
  handleSelectReport: (report: Report) => void
  handleDeselectReport: () => void
  handleViewUser: (user: ViewingUserType) => void
  handleCloseUserProfile: () => void
  handleAssign: () => Promise<void>
  handleUnassign: () => Promise<void>
  handleResolveClick: (status: "resolved" | "dismissed", shiftKey: boolean) => Promise<void>
  handleResolveSubmit: () => Promise<void>
  handleCloseResolveModal: () => void
  handleRefreshNewReports: () => void
  handlePrevPage: () => void
  handleNextPage: () => void
}

export const useReportsActions = (config: UseReportsActionsConfig): UseReportsActionsResult => {
  const {
    selectedReport,
    selectedStatusRef,
    resolveAction,
    resolutionNote,
    reasons,
    updateReport,
    removeReport,
    setSelectedReport,
    setSelectedStatus,
    setSelectedType,
    setSelectedReason,
    setPage,
    clearNewReports,
    setViewingUser,
    setProfileKey,
    setShowResolveModal,
    setResolveAction,
    setResolutionNote,
    setError,
    loadReports,
    pushState,
    isNavigating,
  } = config

  const statusOptions: SelectOption[] = useMemo(() => [
    { value: "active", label: i18n._("reports.filter.active", { message: "Active" }) },
    { value: "pending", label: i18n._("reports.filter.pending", { message: "Pending" }) },
    { value: "in_review", label: i18n._("reports.filter.inReview", { message: "In Review" }) },
    { value: "resolved", label: i18n._("reports.filter.resolved", { message: "Resolved" }) },
    { value: "dismissed", label: i18n._("reports.filter.dismissed", { message: "Dismissed" }) },
    { value: "", label: i18n._("reports.filter.all", { message: "All" }) },
  ], [])

  const typeOptions: SelectOption[] = useMemo(() => [
    { value: "", label: i18n._("reports.filter.allTypes", { message: "All Types" }) },
    { value: "post", label: i18n._("reports.filter.post", { message: "Posts" }) },
    { value: "comment", label: i18n._("reports.filter.comment", { message: "Comments" }) },
  ], [])

  const reasonOptions: SelectOption[] = useMemo(() => [
    { value: "", label: "All Reasons" },
    ...reasons.map((r) => ({ value: r.title, label: r.title })),
  ], [reasons])

  const handleStatusChange = useCallback((option?: SelectOption) => {
    setSelectedStatus((option?.value || "active") as ReportStatus | "active")
    setPage(1)
  }, [setSelectedStatus, setPage])

  const handleTypeChange = useCallback((option?: SelectOption) => {
    setSelectedType((option?.value || "") as ReportType | "")
    setPage(1)
  }, [setSelectedType, setPage])

  const handleReasonChange = useCallback((option?: SelectOption) => {
    setSelectedReason(option?.value || "")
    setPage(1)
  }, [setSelectedReason, setPage])

  const handleSelectReport = useCallback((report: Report) => {
    if (!isNavigating.current) {
      pushState({ reportId: report.id })
    }
    setSelectedReport(report)
    setViewingUser(null)
  }, [isNavigating, pushState, setSelectedReport, setViewingUser])

  const handleDeselectReport = useCallback(() => {
    if (!isNavigating.current) {
      pushState({})
    }
    setSelectedReport(null)
    setViewingUser(null)
  }, [isNavigating, pushState, setSelectedReport, setViewingUser])

  const handleViewUser = useCallback((user: ViewingUserType) => {
    if (!isNavigating.current && selectedReport) {
      pushState({ reportId: selectedReport.id, userId: user.id })
    }
    setViewingUser(user)
    setProfileKey((prev) => prev + 1)
  }, [isNavigating, pushState, selectedReport, setViewingUser, setProfileKey])

  const handleCloseUserProfile = useCallback(() => {
    if (!isNavigating.current && selectedReport) {
      pushState({ reportId: selectedReport.id })
    }
    setViewingUser(null)
  }, [isNavigating, pushState, selectedReport, setViewingUser])

  const handleAssign = useCallback(async () => {
    if (!selectedReport) return
    const result = await actions.assignReport(selectedReport.id)
    if (result.ok) {
      const updatedReport = {
        ...selectedReport,
        status: "in_review" as ReportStatus,
        assignedTo: Fider.session.user,
      }
      setSelectedReport(updatedReport)
      updateReport(updatedReport)
    }
  }, [selectedReport, setSelectedReport, updateReport])

  const handleUnassign = useCallback(async () => {
    if (!selectedReport) return
    const result = await actions.unassignReport(selectedReport.id)
    if (result.ok) {
      const updatedReport = {
        ...selectedReport,
        status: "pending" as ReportStatus,
        assignedTo: undefined,
      }
      setSelectedReport(updatedReport)
      updateReport(updatedReport)
    }
  }, [selectedReport, setSelectedReport, updateReport])

  const handleResolveClick = useCallback(async (status: "resolved" | "dismissed", shiftKey: boolean) => {
    if (!selectedReport) return

    if (shiftKey) {
      const result = await actions.resolveReport(selectedReport.id, status)
      if (result.ok) {
        if (selectedStatusRef.current === "active") {
          removeReport(selectedReport.id)
        } else {
          updateReport({ ...selectedReport, status })
        }
        setSelectedReport(null)
      }
      return
    }

    setResolveAction(status)
    setResolutionNote("")
    setShowResolveModal(true)
  }, [selectedReport, selectedStatusRef, updateReport, removeReport, setSelectedReport, setResolveAction, setResolutionNote, setShowResolveModal])

  const handleResolveSubmit = useCallback(async () => {
    if (!selectedReport) return

    setError(undefined)
    const result = await actions.resolveReport(selectedReport.id, resolveAction, resolutionNote)
    if (result.ok) {
      setShowResolveModal(false)
      if (selectedStatusRef.current === "active") {
        removeReport(selectedReport.id)
      } else {
        updateReport({ ...selectedReport, status: resolveAction })
      }
      setSelectedReport(null)
    } else {
      setError(result.error)
    }
  }, [selectedReport, resolveAction, resolutionNote, selectedStatusRef, updateReport, removeReport, setSelectedReport, setShowResolveModal, setError])

  const handleCloseResolveModal = useCallback(() => {
    setShowResolveModal(false)
  }, [setShowResolveModal])

  const handleRefreshNewReports = useCallback(() => {
    clearNewReports()
    loadReports()
  }, [clearNewReports, loadReports])

  const handlePrevPage = useCallback(() => {
    setPage((p) => p - 1)
  }, [setPage])

  const handleNextPage = useCallback(() => {
    setPage((p) => p + 1)
  }, [setPage])

  return {
    statusOptions,
    typeOptions,
    reasonOptions,
    handleStatusChange,
    handleTypeChange,
    handleReasonChange,
    handleSelectReport,
    handleDeselectReport,
    handleViewUser,
    handleCloseUserProfile,
    handleAssign,
    handleUnassign,
    handleResolveClick,
    handleResolveSubmit,
    handleCloseResolveModal,
    handleRefreshNewReports,
    handlePrevPage,
    handleNextPage,
  }
}
