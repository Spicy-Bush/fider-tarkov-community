import { useState, useCallback, useRef, useEffect, useMemo } from "react"
import { Report, ReportStatus, ReportType, ReportReason, Post, DiscussionComment, UserRole, UserStatus } from "@fider/models"
import { actions, Failure, PAGINATION } from "@fider/services"
import { ReportViewers } from "@fider/services/actions/report"

export interface ViewingUserType {
  id: number
  name: string
  avatarURL: string
  role: number | UserRole
  status: number | UserStatus
}

interface UseReportsStateResult {
  reports: Report[]
  updateReport: (report: Report) => void
  removeReport: (reportId: number) => void
  total: number
  setTotal: React.Dispatch<React.SetStateAction<number>>
  page: number
  setPage: React.Dispatch<React.SetStateAction<number>>
  perPage: number
  isLoading: boolean
  selectedStatus: ReportStatus | "active"
  setSelectedStatus: React.Dispatch<React.SetStateAction<ReportStatus | "active">>
  selectedType: ReportType | ""
  setSelectedType: React.Dispatch<React.SetStateAction<ReportType | "">>
  selectedReason: string
  setSelectedReason: React.Dispatch<React.SetStateAction<string>>
  reasons: ReportReason[]
  selectedReport: Report | null
  setSelectedReport: React.Dispatch<React.SetStateAction<Report | null>>
  selectedReportRef: React.MutableRefObject<Report | null>
  selectedStatusRef: React.MutableRefObject<ReportStatus | "active">
  previewPost: Post | null
  previewComment: DiscussionComment | null
  isLoadingPreview: boolean
  showResolveModal: boolean
  setShowResolveModal: React.Dispatch<React.SetStateAction<boolean>>
  resolveAction: "resolved" | "dismissed"
  setResolveAction: React.Dispatch<React.SetStateAction<"resolved" | "dismissed">>
  resolutionNote: string
  setResolutionNote: React.Dispatch<React.SetStateAction<string>>
  error: Failure | undefined
  setError: React.Dispatch<React.SetStateAction<Failure | undefined>>
  newReportIds: Set<number>
  clearNewReports: () => void
  viewingUser: ViewingUserType | null
  setViewingUser: React.Dispatch<React.SetStateAction<ViewingUserType | null>>
  profileKey: number
  setProfileKey: React.Dispatch<React.SetStateAction<number>>
  viewers: ReportViewers[]
  loadReports: (options?: { markNew: boolean }) => Promise<void>
  loadPreviewContent: (reportId: number) => Promise<void>
}

export const useReportsState = (): UseReportsStateResult => {
  const perPage = PAGINATION.REPORTS_LIMIT

  const [reportRows, setReportRows] = useState<{ report: Report; isNew: boolean }[]>()
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [isLoading, setIsLoading] = useState(true)
  const [selectedStatus, setSelectedStatus] = useState<ReportStatus | "active">("active")
  const [selectedType, setSelectedType] = useState<ReportType | "">("")
  const [selectedReason, setSelectedReason] = useState<string>("")
  const [reasons, setReasons] = useState<ReportReason[]>([])
  const [selectedReport, setSelectedReport] = useState<Report | null>(null)
  const [previewPost, setPreviewPost] = useState<Post | null>(null)
  const [previewComment, setPreviewComment] = useState<DiscussionComment | null>(null)
  const [isLoadingPreview, setIsLoadingPreview] = useState(false)
  const [showResolveModal, setShowResolveModal] = useState(false)
  const [resolveAction, setResolveAction] = useState<"resolved" | "dismissed">("resolved")
  const [resolutionNote, setResolutionNote] = useState("")
  const [error, setError] = useState<Failure | undefined>()
  const [viewingUser, setViewingUser] = useState<ViewingUserType | null>(null)
  const [profileKey, setProfileKey] = useState(0)
  const [viewers, setViewers] = useState<ReportViewers[]>([])

  const selectedReportRef = useRef<Report | null>(null)
  const selectedStatusRef = useRef<ReportStatus | "active">("active")
  const reportsRequest = useRef<AbortController>()
  const previewRequest = useRef<AbortController>()
  const reports = useMemo(() => reportRows?.map((row) => row.report) ?? [], [reportRows])
  const newReportIds = new Set(reportRows?.filter((row) => row.isNew).map((row) => row.report.id))

  useEffect(() => {
    setReportRows(undefined)

    return () => reportsRequest.current?.abort()
  }, [page, selectedStatus, selectedType, selectedReason])

  useEffect(() => {
    setIsLoadingPreview(false)

    if (selectedReport) {
      setReportRows((previous) => previous?.map((row) => {
        if (row.report.id === selectedReport.id && row.isNew) {
          return { ...row, isNew: false }
        }

        return row
      }))
    }

    return () => previewRequest.current?.abort()
  }, [selectedReport?.id])

  useEffect(() => {
    selectedReportRef.current = selectedReport
  }, [selectedReport])

  useEffect(() => {
    selectedStatusRef.current = selectedStatus
  }, [selectedStatus])

  useEffect(() => {
    const loadReasons = async () => {
      const result = await actions.getReportReasons()
      if (result.ok) {
        setReasons(result.data)
      }
    }
    loadReasons()
  }, [])

  const loadReports = useCallback(async (options?: { markNew: boolean }) => {
    reportsRequest.current?.abort()
    const request = new AbortController()
    reportsRequest.current = request
    setIsLoading(true)

    const params: {
      page: number
      perPage: number
      status?: ReportStatus | ReportStatus[]
      type?: ReportType
      reason?: string
    } = {
      page,
      perPage,
    }
    if (selectedStatus === "active") {
      params.status = ["pending", "in_review"] as ReportStatus[]
    } else if (selectedStatus) {
      params.status = selectedStatus as ReportStatus
    }
    if (selectedType) params.type = selectedType as ReportType
    if (selectedReason) params.reason = selectedReason

    try {
      const result = await actions.listReports(params, request.signal)

      if (request.signal.aborted) {
        return
      }

      if (result.ok) {
        const markNew = options?.markNew && (selectedStatus === "active" || selectedStatus === "pending")
        const selectedId = selectedReportRef.current?.id

        setReportRows((previous) => {
          if (!markNew || !previous) {
            return result.data.reports.map((report) => ({ report, isNew: false }))
          }

          const known = new Map(previous.map((row) => [row.report.id, row.isNew]))

          return result.data.reports.map((report) => ({
            report,
            isNew: report.id !== selectedId && (known.get(report.id) ?? true),
          }))
        })
        setTotal(result.data.total)
        setViewers(result.data.viewers)
      } else {
        setError(result.error)
      }
    } catch (cause) {
      if (!request.signal.aborted) {
        setError({ errors: [{ message: "Could not refresh reports." }], cause })
      }
    } finally {
      if (!request.signal.aborted) {
        setIsLoading(false)
      }
    }
  }, [page, selectedStatus, selectedType, selectedReason, perPage])

  const clearNewReports = useCallback(() => {
    setReportRows((previous) => previous?.map((row) => {
      if (row.isNew) {
        return { ...row, isNew: false }
      }

      return row
    }))
  }, [])

  const updateReport = useCallback((report: Report) => {
    setReportRows((previous) => previous?.map((row) => {
      if (row.report.id === report.id) {
        return { ...row, report }
      }

      return row
    }))
  }, [])

  const removeReport = useCallback((reportId: number) => {
    setReportRows((previous) => previous?.filter((row) => row.report.id !== reportId))
  }, [])

  const loadPreviewContent = useCallback(async (reportId: number) => {
    previewRequest.current?.abort()
    const request = new AbortController()
    previewRequest.current = request
    const isCurrent = () => !request.signal.aborted && selectedReportRef.current?.id === reportId

    const loadingTimeout = setTimeout(() => {
      if (isCurrent()) {
        setIsLoadingPreview(true)
      }
    }, 150)

    try {
      const result = await actions.getReportDetails(reportId, request.signal)

      if (!isCurrent()) {
        return
      }

      if (result.ok) {
        const freshReport = result.data.report
        if (selectedStatusRef.current === "active" && (freshReport.status === "resolved" || freshReport.status === "dismissed")) {
          setSelectedReport(null)
          setPreviewPost(null)
          setPreviewComment(null)
          return
        }

        const current = selectedReportRef.current!
        if (current.status !== freshReport.status || current.assignedTo?.id !== freshReport.assignedTo?.id) {
          setSelectedReport(freshReport)
          updateReport(freshReport)
        }
        setPreviewPost(result.data.post || null)
        setPreviewComment(result.data.comment || null)
      } else {
        setPreviewPost(null)
        setPreviewComment(null)
        setSelectedReport(null)
        setError(result.error)
      }
    } catch (cause) {
      if (isCurrent()) {
        setError({ errors: [{ message: "Could not refresh the report preview." }], cause })
      }
    } finally {
      clearTimeout(loadingTimeout)
      if (isCurrent()) {
        setIsLoadingPreview(false)
      }
    }
  }, [updateReport])

  return {
    reports,
    updateReport,
    removeReport,
    total,
    setTotal,
    page,
    setPage,
    perPage,
    isLoading,
    selectedStatus,
    setSelectedStatus,
    selectedType,
    setSelectedType,
    selectedReason,
    setSelectedReason,
    reasons,
    selectedReport,
    setSelectedReport,
    selectedReportRef,
    selectedStatusRef,
    previewPost,
    previewComment,
    isLoadingPreview,
    showResolveModal,
    setShowResolveModal,
    resolveAction,
    setResolveAction,
    resolutionNote,
    setResolutionNote,
    error,
    setError,
    newReportIds,
    clearNewReports,
    viewingUser,
    setViewingUser,
    profileKey,
    setProfileKey,
    viewers,
    loadReports,
    loadPreviewContent,
  }
}
