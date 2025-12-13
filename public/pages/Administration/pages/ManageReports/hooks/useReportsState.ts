import { useState, useCallback, useRef, useEffect } from "react"
import { Report, ReportStatus, ReportType, ReportReason, Post, Comment, UserRole, UserStatus } from "@fider/models"
import { actions, Failure, PAGINATION } from "@fider/services"

export interface ViewingUserType {
  id: number
  name: string
  avatarURL: string
  role: number | UserRole
  status: number | UserStatus
}

interface UseReportsStateResult {
  reports: Report[]
  setReports: React.Dispatch<React.SetStateAction<Report[]>>
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
  previewComment: Comment | null
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
  setNewReportIds: React.Dispatch<React.SetStateAction<Set<number>>>
  viewingUser: ViewingUserType | null
  setViewingUser: React.Dispatch<React.SetStateAction<ViewingUserType | null>>
  profileKey: number
  setProfileKey: React.Dispatch<React.SetStateAction<number>>
  loadReports: () => Promise<void>
  loadPreviewContent: (reportId: number) => Promise<void>
}

export const useReportsState = (): UseReportsStateResult => {
  const perPage = PAGINATION.REPORTS_LIMIT

  const [reports, setReports] = useState<Report[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [isLoading, setIsLoading] = useState(true)
  const [selectedStatus, setSelectedStatus] = useState<ReportStatus | "active">("active")
  const [selectedType, setSelectedType] = useState<ReportType | "">("")
  const [selectedReason, setSelectedReason] = useState<string>("")
  const [reasons, setReasons] = useState<ReportReason[]>([])
  const [selectedReport, setSelectedReport] = useState<Report | null>(null)
  const [previewPost, setPreviewPost] = useState<Post | null>(null)
  const [previewComment, setPreviewComment] = useState<Comment | null>(null)
  const [isLoadingPreview, setIsLoadingPreview] = useState(false)
  const [showResolveModal, setShowResolveModal] = useState(false)
  const [resolveAction, setResolveAction] = useState<"resolved" | "dismissed">("resolved")
  const [resolutionNote, setResolutionNote] = useState("")
  const [error, setError] = useState<Failure | undefined>()
  const [newReportIds, setNewReportIds] = useState<Set<number>>(new Set())
  const [viewingUser, setViewingUser] = useState<ViewingUserType | null>(null)
  const [profileKey, setProfileKey] = useState(0)

  const selectedReportRef = useRef<Report | null>(null)
  const selectedStatusRef = useRef<ReportStatus | "active">("active")

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

  const loadReports = useCallback(async () => {
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

    const result = await actions.listReports(params)
    if (result.ok) {
      setReports(result.data.reports || [])
      setTotal(result.data.total)
    }
    setIsLoading(false)
  }, [page, selectedStatus, selectedType, selectedReason, perPage])

  const loadPreviewContent = useCallback(async (reportId: number) => {
    const loadingTimeout = setTimeout(() => {
      if (selectedReportRef.current?.id === reportId) {
        setIsLoadingPreview(true)
      }
    }, 150)

    try {
      const result = await actions.getReportDetails(reportId)
      if (selectedReportRef.current?.id !== reportId) return

      if (result.ok) {
        const freshReport = result.data.report
        const current = selectedReportRef.current
        if (current.status !== freshReport.status || current.assignedTo?.id !== freshReport.assignedTo?.id) {
          setSelectedReport(freshReport)
          setReports((prev) => prev.map((r) => (r.id === reportId ? freshReport : r)))
        }
        setPreviewPost(result.data.post || null)
        setPreviewComment(result.data.comment || null)
      }
    } finally {
      clearTimeout(loadingTimeout)
      if (selectedReportRef.current?.id === reportId) {
        setIsLoadingPreview(false)
      }
    }
  }, [])

  return {
    reports,
    setReports,
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
    setNewReportIds,
    viewingUser,
    setViewingUser,
    profileKey,
    setProfileKey,
    loadReports,
    loadPreviewContent,
  }
}

