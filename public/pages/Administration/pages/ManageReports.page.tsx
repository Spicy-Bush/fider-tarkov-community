import React, { useState, useEffect, useCallback, useRef } from "react"
import {
  Button,
  Form,
  Loader,
  Select,
  SelectOption,
  Modal,
  Icon,
  Avatar,
  TextArea,
  Moment,
  Markdown,
} from "@fider/components"
import { HStack, VStack } from "@fider/components/layout"
import { actions, Failure, Fider, classSet, modEventSource } from "@fider/services"
import { AdminPageContainer } from "../components/AdminBasePage"
import IconCheck from "@fider/assets/images/heroicons-check.svg"
import IconX from "@fider/assets/images/heroicons-x.svg"
import IconRefresh from "@fider/assets/images/heroicons-refresh.svg"
import IconExternalLink from "@fider/assets/images/heroicons-external-link.svg"
import IconEye from "@fider/assets/images/heroicons-eye.svg"
import { Report, ReportStatus, ReportType, ReportReason, Post, Comment, getReportTypeLabel } from "@fider/models"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"

import "./ManageReports.page.scss"

interface Viewer {
  userId: number
  userName: string
}

interface ReportListItemProps {
  report: Report
  isSelected: boolean
  onClick: () => void
  viewers: Viewer[]
}

const ReportListItem: React.FC<ReportListItemProps> = ({ report, isSelected, onClick, viewers }) => {
  const className = classSet({
    "c-report-list-item": true,
    "c-report-list-item--selected": isSelected,
    "c-report-list-item--pending": report.status === "pending",
    "c-report-list-item--in-review": report.status === "in_review",
  })

  const otherViewers = viewers.filter((v) => v.userId !== Fider.session.user.id)

  return (
    <div className={className} onClick={onClick}>
      <div className="c-report-list-item__header">
        <span className="c-report-list-item__type">{getReportTypeLabel(report.reportedType)}</span>
        <HStack spacing={1}>
          {otherViewers.length > 0 && (
            <span className="c-report-list-item__viewers" data-tooltip={otherViewers.map((v) => v.userName).join(", ")}>
              <Icon sprite={IconEye} className="h-3" />
              <span>{otherViewers.length}</span>
            </span>
          )}
          <span className="c-report-list-item__status">{report.status.replace("_", " ")}</span>
        </HStack>
      </div>
      <div className="c-report-list-item__reason">{report.reason}</div>
      <div className="c-report-list-item__meta">
        <Avatar user={report.reporter} />
        <span>{report.reporter.name}</span>
        <Moment locale={Fider.currentLocale} date={report.createdAt} />
      </div>
    </div>
  )
}

interface ContentPreviewProps {
  report: Report | null
  post: Post | null
  comment: Comment | null
  isLoading: boolean
  onAssign: () => void
  onUnassign: () => void
  onResolve: (status: "resolved" | "dismissed", shiftKey: boolean) => void
  currentUserId: number
}

const ContentPreview: React.FC<ContentPreviewProps> = ({
  report,
  post,
  comment,
  isLoading,
  onAssign,
  onUnassign,
  onResolve,
  currentUserId,
}) => {
  if (!report) {
    return (
      <div className="c-report-detail c-report-detail--empty">
        <div className="c-report-detail__empty-state">
          <p className="text-muted">Select a report to view details</p>
        </div>
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="c-report-detail c-report-detail--loading">
        <Loader />
      </div>
    )
  }

  const isAssignedToMe = report.assignedTo?.id === currentUserId
  const canAction = report.status === "pending" || report.status === "in_review"

  const getTargetLink = () => {
    if (!report.postNumber || !report.postSlug) return "#"
    if (report.reportedType === "post") {
      return `/posts/${report.postNumber}/${report.postSlug}`
    }
    if (report.reportedType === "comment") {
      return `/posts/${report.postNumber}/${report.postSlug}#comment-${report.reportedId}`
    }
    return "#"
  }

  const reportedUser = report.reportedType === "post" ? post?.user : comment?.user

  return (
    <div className="c-report-detail">
      <div className="c-report-detail__actions">
        {canAction && (
          <HStack spacing={2}>
            {report.status === "pending" && (
              <Button size="small" variant="secondary" onClick={onAssign}>
                Assign to me
              </Button>
            )}
            {report.status === "in_review" && isAssignedToMe && (
              <Button size="small" variant="secondary" onClick={onUnassign}>
                Unassign
              </Button>
            )}
            <span onClickCapture={(e) => { if (e.shiftKey) { e.stopPropagation(); onResolve("resolved", true) } }}>
              <Button size="small" variant="primary" onClick={() => onResolve("resolved", false)}>
                <Icon sprite={IconCheck} className="h-4" />
                <span>Resolve</span>
              </Button>
            </span>
            <span onClickCapture={(e) => { if (e.shiftKey) { e.stopPropagation(); onResolve("dismissed", true) } }}>
              <Button size="small" variant="danger" onClick={() => onResolve("dismissed", false)}>
                <Icon sprite={IconX} className="h-4" />
                <span>Dismiss</span>
              </Button>
            </span>
          </HStack>
        )}
        {!canAction && (
          <span className={`c-report-detail__status-badge c-report-detail__status-badge--${report.status}`}>
            {report.status === "resolved" ? "Resolved" : "Dismissed"}
          </span>
        )}
      </div>

      <div className="c-report-detail__section">
        <h4 className="c-report-detail__section-title">Report Information</h4>
        <div className="c-report-detail__grid">
          <div className="c-report-detail__field">
            <label>Reason</label>
            <span className="c-report-detail__reason-badge">{report.reason}</span>
          </div>
          <div className="c-report-detail__field">
            <label>Reported by</label>
            <HStack spacing={2}>
              <Avatar user={report.reporter} />
              <div>
                <div className="font-medium">{report.reporter.name}</div>
                <div className="text-xs text-muted">
                  <Moment locale={Fider.currentLocale} date={report.createdAt} />
                </div>
              </div>
            </HStack>
          </div>
        </div>
        {report.details && (
          <div className="c-report-detail__details">
            <label>Additional Details</label>
            <div className="c-report-detail__details-content">{report.details}</div>
          </div>
        )}
      </div>

      <div className="c-report-detail__section">
        <div className="c-report-detail__section-header">
          <h4 className="c-report-detail__section-title">
            Reported {report.reportedType === "post" ? "Post" : "Comment"}
          </h4>
          <a
            href={getTargetLink()}
            target="_blank"
            rel="noopener noreferrer"
            className="c-report-detail__external-link"
          >
            <Icon sprite={IconExternalLink} className="h-4" />
            <span>View original</span>
          </a>
        </div>

        {reportedUser && (
          <div className="c-report-detail__author">
            <HStack spacing={2}>
              <Avatar user={reportedUser} />
              <div>
                <div className="font-medium">{reportedUser.name}</div>
                <div className="text-xs text-muted">
                  {report.reportedType === "post" && post && (
                    <Moment locale={Fider.currentLocale} date={post.createdAt} />
                  )}
                  {report.reportedType === "comment" && comment && (
                    <Moment locale={Fider.currentLocale} date={comment.createdAt} />
                  )}
                </div>
              </div>
            </HStack>
          </div>
        )}

        <div className="c-report-detail__content">
          {report.reportedType === "post" && post && (
            <>
              <h5 className="c-report-detail__post-title">{post.title}</h5>
              {post.description && (
                <div className="c-report-detail__post-body">
                  <Markdown text={post.description} style="full" />
                </div>
              )}
            </>
          )}

          {report.reportedType === "comment" && comment && (
            <>
              <div className="c-report-detail__comment-body">
                <Markdown text={comment.content} style="full" />
              </div>
              {post && (
                <div className="c-report-detail__comment-context">
                  On post: <a href={`/posts/${post.number}/${post.slug}`} target="_blank" rel="noopener noreferrer">{post.title}</a>
                </div>
              )}
            </>
          )}

          {!post && !comment && (
            <p className="text-muted">Content not found or has been deleted.</p>
          )}
        </div>
      </div>

      {report.assignedTo && (
        <div className="c-report-detail__section c-report-detail__section--assignment">
          <h4 className="c-report-detail__section-title">Assignment</h4>
          <div className="c-report-detail__field">
            <label>Assigned to</label>
            <HStack spacing={2}>
              <Avatar user={report.assignedTo} />
              <span className="font-medium">{report.assignedTo?.name}</span>
            </HStack>
          </div>
        </div>
      )}
    </div>
  )
}

const ManageReportsContent: React.FC = () => {
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
  const [viewers, setViewers] = useState<Map<number, Viewer[]>>(new Map())
  const [newReportIds, setNewReportIds] = useState<Set<number>>(new Set())
  const selectedReportRef = useRef<Report | null>(null)
  const selectedStatusRef = useRef<ReportStatus | "active">("active")

  useEffect(() => {
    selectedReportRef.current = selectedReport
  }, [selectedReport])

  useEffect(() => {
    selectedStatusRef.current = selectedStatus
  }, [selectedStatus])

  useEffect(() => {
    modEventSource.connect()

    const unsubNewReport = modEventSource.on("report.new", (_, payload) => {
      const data = payload as { reportId: number; reportedType: string; reportedId: number; reason: string }
      if (selectedStatusRef.current === "active" || selectedStatusRef.current === "pending") {
        setNewReportIds((prev) => new Set(prev).add(data.reportId))
      }
    })

    const unsubAssigned = modEventSource.on("report.assigned", (_, payload) => {
      const data = payload as { reportId: number; assignedTo: { userId: number; userName: string } }
      const assignedUser = { id: data.assignedTo.userId, name: data.assignedTo.userName } as Report["assignedTo"]
      
      setReports((prev) =>
        prev.map((r) =>
          r.id === data.reportId
            ? { ...r, status: "in_review" as ReportStatus, assignedTo: assignedUser }
            : r
        )
      )
      if (selectedReportRef.current?.id === data.reportId) {
        setSelectedReport((prev) =>
          prev ? { ...prev, status: "in_review" as ReportStatus, assignedTo: assignedUser } : prev
        )
      }
    })

    const unsubUnassigned = modEventSource.on("report.unassigned", (_, payload) => {
      const data = payload as { reportId: number }
      
      setReports((prev) =>
        prev.map((r) =>
          r.id === data.reportId
            ? { ...r, status: "pending" as ReportStatus, assignedTo: undefined }
            : r
        )
      )
      if (selectedReportRef.current?.id === data.reportId) {
        setSelectedReport((prev) =>
          prev ? { ...prev, status: "pending" as ReportStatus, assignedTo: undefined } : prev
        )
      }
    })

    const unsubResolved = modEventSource.on("report.resolved", (_, payload) => {
      const data = payload as { reportId: number; status: string }
      if (selectedStatusRef.current === "active") {
        setReports((prev) => prev.filter((r) => r.id !== data.reportId))
        if (selectedReportRef.current?.id === data.reportId) {
          setSelectedReport(null)
        }
      } else {
        setReports((prev) =>
          prev.map((r) =>
            r.id === data.reportId
              ? { ...r, status: data.status as ReportStatus }
              : r
          )
        )
      }
    })

    const unsubViewerJoined = modEventSource.on("report.viewer_joined", (_, payload) => {
      const data = payload as { reportId: number; userId: number; userName: string }
      setViewers((prev) => {
        const newMap = new Map(prev)
        const current = newMap.get(data.reportId) || []
        if (!current.find((v) => v.userId === data.userId)) {
          newMap.set(data.reportId, [...current, { userId: data.userId, userName: data.userName }])
        }
        return newMap
      })
    })

    const unsubViewerLeft = modEventSource.on("report.viewer_left", (_, payload) => {
      const data = payload as { reportId: number; userId: number }
      setViewers((prev) => {
        const newMap = new Map(prev)
        const current = newMap.get(data.reportId) || []
        newMap.set(data.reportId, current.filter((v) => v.userId !== data.userId))
        return newMap
      })
    })

    return () => {
      modEventSource.disconnect()
      unsubNewReport()
      unsubAssigned()
      unsubUnassigned()
      unsubResolved()
      unsubViewerJoined()
      unsubViewerLeft()
    }
  }, [])

  const loadReports = useCallback(async () => {
    setIsLoading(true)
    const params: { page: number; perPage: number; status?: ReportStatus | ReportStatus[]; type?: ReportType; reason?: string } = {
      page,
      perPage: 50,
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
      if (result.data.viewers) {
        const viewerMap = new Map<number, Viewer[]>()
        for (const rv of result.data.viewers) {
          viewerMap.set(rv.reportId, rv.viewers.map(v => ({ userId: v.userId, userName: v.userName })))
        }
        setViewers(viewerMap)
      }
    }
    setIsLoading(false)
  }, [page, selectedStatus, selectedType, selectedReason])

  useEffect(() => {
    loadReasons()
  }, [])

  useEffect(() => {
    loadReports()
  }, [loadReports])

  const loadReasons = async () => {
    const result = await actions.getReportReasons()
    if (result.ok) {
      setReasons(result.data)
    }
  }

  useEffect(() => {
    if (selectedReport) {
      loadPreviewContent(selectedReport.id)
    } else {
      setPreviewPost(null)
      setPreviewComment(null)
      modEventSource.stopViewingReport()
    }
  }, [selectedReport?.id])

  const loadPreviewContent = async (reportId: number) => {
    const loadingTimeout = setTimeout(() => {
      if (selectedReportRef.current?.id === reportId) {
        setIsLoadingPreview(true)
      }
    }, 150)

    try {
      const result = await actions.getReportDetails(reportId)
      
      if (selectedReportRef.current?.id !== reportId) {
        return
      }

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
  }

  const handleStatusChange = (option?: SelectOption) => {
    setSelectedStatus((option?.value || "active") as ReportStatus | "active")
    setPage(1)
  }

  const handleTypeChange = (option?: SelectOption) => {
    setSelectedType((option?.value || "") as ReportType | "")
    setPage(1)
  }

  const handleReasonChange = (option?: SelectOption) => {
    setSelectedReason(option?.value || "")
    setPage(1)
  }

  const handleSelectReport = (report: Report) => {
    setSelectedReport(report)
    setNewReportIds((prev) => {
      const newSet = new Set(prev)
      newSet.delete(report.id)
      return newSet
    })
    modEventSource.viewReport(report.id)
  }

  const handleAssign = async () => {
    if (!selectedReport) return
    const result = await actions.assignReport(selectedReport.id)
    if (result.ok) {
      const updatedReport = { ...selectedReport, status: "in_review" as ReportStatus, assignedTo: Fider.session.user }
      setSelectedReport(updatedReport)
      setReports((prev) => prev.map((r) => (r.id === selectedReport.id ? updatedReport : r)))
    }
  }

  const handleUnassign = async () => {
    if (!selectedReport) return
    const result = await actions.unassignReport(selectedReport.id)
    if (result.ok) {
      const updatedReport = { ...selectedReport, status: "pending" as ReportStatus, assignedTo: undefined }
      setSelectedReport(updatedReport)
      setReports((prev) => prev.map((r) => (r.id === selectedReport.id ? updatedReport : r)))
    }
  }

  const handleResolveClick = async (status: "resolved" | "dismissed", shiftKey: boolean) => {
    if (!selectedReport) return

    if (shiftKey) {
      const result = await actions.resolveReport(selectedReport.id, status)
      if (result.ok) {
        if (selectedStatusRef.current === "active") {
          setReports((prev) => prev.filter((r) => r.id !== selectedReport.id))
        } else {
          setReports((prev) =>
            prev.map((r) => (r.id === selectedReport.id ? { ...r, status: status as ReportStatus } : r))
          )
        }
        setSelectedReport(null)
      }
      return
    }
    
    setResolveAction(status)
    setResolutionNote("")
    setShowResolveModal(true)
  }

  const handleResolveSubmit = async () => {
    if (!selectedReport) return

    setError(undefined)
    const result = await actions.resolveReport(selectedReport.id, resolveAction, resolutionNote)
    if (result.ok) {
      setShowResolveModal(false)
      if (selectedStatusRef.current === "active") {
        setReports((prev) => prev.filter((r) => r.id !== selectedReport.id))
      } else {
        setReports((prev) =>
          prev.map((r) => (r.id === selectedReport.id ? { ...r, status: resolveAction as ReportStatus } : r))
        )
      }
      setSelectedReport(null)
    } else {
      setError(result.error)
    }
  }

  const statusOptions: SelectOption[] = [
    { value: "active", label: i18n._("reports.filter.active", { message: "Active" }) },
    { value: "pending", label: i18n._("reports.filter.pending", { message: "Pending" }) },
    { value: "in_review", label: i18n._("reports.filter.inReview", { message: "In Review" }) },
    { value: "resolved", label: i18n._("reports.filter.resolved", { message: "Resolved" }) },
    { value: "dismissed", label: i18n._("reports.filter.dismissed", { message: "Dismissed" }) },
    { value: "", label: i18n._("reports.filter.all", { message: "All" }) },
  ]

  const typeOptions: SelectOption[] = [
    { value: "", label: i18n._("reports.filter.allTypes", { message: "All Types" }) },
    { value: "post", label: i18n._("reports.filter.post", { message: "Posts" }) },
    { value: "comment", label: i18n._("reports.filter.comment", { message: "Comments" }) },
  ]

  const reasonOptions: SelectOption[] = [
    { value: "", label: "All Reasons" },
    ...reasons.map((r) => ({ value: r.title, label: r.title })),
  ]

  return (
    <div className="c-reports-split-view">
      <div className="c-reports-split-view__list">
        <div className="c-reports-split-view__filters">
          <VStack spacing={2}>
            <HStack spacing={2}>
              <Select
                field="status"
                defaultValue={selectedStatus}
                options={statusOptions}
                onChange={handleStatusChange}
              />
              <Select
                field="type"
                defaultValue={selectedType}
                options={typeOptions}
                onChange={handleTypeChange}
              />
            </HStack>
            <HStack spacing={2}>
              <Select
                field="reason"
                defaultValue={selectedReason}
                options={reasonOptions}
                onChange={handleReasonChange}
              />
              <Button variant="secondary" size="small" onClick={() => loadReports()}>
                <Icon sprite={IconRefresh} className="h-4" />
              </Button>
            </HStack>
          </VStack>
        </div>

        <div className="c-reports-split-view__list-content">
          {newReportIds.size > 0 && (
            <button
              className="c-reports-new-banner"
              onClick={() => {
                setNewReportIds(new Set())
                loadReports()
              }}
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
                  onClick={() => handleSelectReport(report)}
                  viewers={viewers.get(report.id) || []}
                />
              ))}
            </VStack>
          )}
        </div>

        {total > reports.length && (
          <div className="c-reports-split-view__pagination">
            <Button size="small" variant="tertiary" disabled={page === 1} onClick={() => setPage((p) => p - 1)}>
              Prev
            </Button>
            <span className="text-muted text-sm">Page {page}</span>
            <Button size="small" variant="tertiary" disabled={reports.length < 50} onClick={() => setPage((p) => p + 1)}>
              Next
            </Button>
          </div>
        )}
      </div>

      <div className="c-reports-split-view__preview">
        <ContentPreview
          report={selectedReport}
          post={previewPost}
          comment={previewComment}
          isLoading={isLoadingPreview}
          onAssign={handleAssign}
          onUnassign={handleUnassign}
          onResolve={handleResolveClick}
          currentUserId={Fider.session.user.id}
        />
      </div>

      <Modal.Window
        isOpen={showResolveModal}
        onClose={() => setShowResolveModal(false)}
        center={true}
        size="small"
      >
        <Modal.Header>
          {resolveAction === "resolved" ? (
            <Trans id="reports.resolve.title">Resolve Report</Trans>
          ) : (
            <Trans id="reports.dismiss.title">Dismiss Report</Trans>
          )}
        </Modal.Header>
        <Modal.Content>
          <Form error={error}>
            <TextArea
              field="resolutionNote"
              label={i18n._("reports.resolution.label", { message: "Resolution Note (optional)" })}
              value={resolutionNote}
              onChange={setResolutionNote}
              placeholder={i18n._("reports.resolution.placeholder", {
                message: "Add any notes about how this was resolved...",
              })}
            />
          </Form>
        </Modal.Content>
        <Modal.Footer>
          <Button
            variant={resolveAction === "resolved" ? "primary" : "danger"}
            onClick={handleResolveSubmit}
          >
            {resolveAction === "resolved" ? (
              <Trans id="action.resolve">Resolve</Trans>
            ) : (
              <Trans id="action.dismiss">Dismiss</Trans>
            )}
          </Button>
          <Button variant="tertiary" onClick={() => setShowResolveModal(false)}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>
    </div>
  )
}

const ManageReportsPage: React.FC = () => {
  return (
    <AdminPageContainer
      id="p-admin-reports"
      name="reports"
      title="Reports"
      subtitle="Review and manage user reports"
    >
      <ManageReportsContent />
    </AdminPageContainer>
  )
}

export default ManageReportsPage
