import React from "react"
import { Button, Loader, Icon, Avatar, Moment, Markdown } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { Fider } from "@fider/services"
import {
  heroiconsCheck as IconCheck,
  heroiconsX as IconX,
  heroiconsExternalLink as IconExternalLink,
} from "@fider/icons.generated"
import { Report, Post, Comment, UserRole, UserStatus } from "@fider/models"

export interface ContentPreviewProps {
  report: Report | null
  post: Post | null
  comment: Comment | null
  isLoading: boolean
  onAssign: () => void
  onUnassign: () => void
  onResolve: (status: "resolved" | "dismissed", shiftKey: boolean) => void
  currentUserId: number
  onUserClick?: (user: {
    id: number
    name: string
    avatarURL: string
    role: number | UserRole
    status: number | UserStatus
  }) => void
}

export const ContentPreview: React.FC<ContentPreviewProps> = ({
  report,
  post,
  comment,
  isLoading,
  onAssign,
  onUnassign,
  onResolve,
  currentUserId,
  onUserClick,
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
            <span
              onClickCapture={(e) => {
                if (e.shiftKey) {
                  e.stopPropagation()
                  onResolve("resolved", true)
                }
              }}
            >
              <Button
                size="small"
                variant="primary"
                onClick={() => onResolve("resolved", false)}
              >
                <Icon sprite={IconCheck} className="h-4" />
                <span>Resolve</span>
              </Button>
            </span>
            <span
              onClickCapture={(e) => {
                if (e.shiftKey) {
                  e.stopPropagation()
                  onResolve("dismissed", true)
                }
              }}
            >
              <Button
                size="small"
                variant="danger"
                onClick={() => onResolve("dismissed", false)}
              >
                <Icon sprite={IconX} className="h-4" />
                <span>Dismiss</span>
              </Button>
            </span>
          </HStack>
        )}
        {!canAction && (
          <span
            className={`c-report-detail__status-badge c-report-detail__status-badge--${report.status}`}
          >
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
            <div
              className="c-report-detail__author c-report-detail__author--clickable"
              onClick={() =>
                onUserClick?.({
                  id: report.reporter.id,
                  name: report.reporter.name,
                  avatarURL: report.reporter.avatarURL,
                  role: report.reporter.role,
                  status: report.reporter.status,
                })
              }
              role="button"
              tabIndex={0}
            >
              <HStack spacing={2}>
                <Avatar user={report.reporter} clickable={false} />
                <div>
                  <div className="font-medium">{report.reporter.name}</div>
                  <div className="text-xs text-muted">
                    <Moment locale={Fider.currentLocale} date={report.createdAt} />
                  </div>
                </div>
              </HStack>
              <span className="c-report-detail__author-hint">View profile</span>
            </div>
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
          <div
            className="c-report-detail__author c-report-detail__author--clickable"
            onClick={() =>
              onUserClick?.({
                id: reportedUser.id,
                name: reportedUser.name,
                avatarURL: reportedUser.avatarURL,
                role: reportedUser.role,
                status: reportedUser.status,
              })
            }
            role="button"
            tabIndex={0}
          >
            <HStack spacing={2}>
              <Avatar user={reportedUser} clickable={false} />
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
            <span className="c-report-detail__author-hint">View profile</span>
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
                  On post:{" "}
                  <a
                    href={`/posts/${post.number}/${post.slug}`}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {post.title}
                  </a>
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
              <Avatar user={report.assignedTo} clickable={false} />
              <span className="font-medium">{report.assignedTo?.name}</span>
            </HStack>
          </div>
        </div>
      )}

      {(report.status === "resolved" || report.status === "dismissed") && (
        <div className={`c-report-detail__section c-report-detail__section--resolution c-report-detail__section--resolution-${report.status}`}>
          <h4 className="c-report-detail__section-title">
            {report.status === "resolved" ? "Resolution Details" : "Dismissal Details"}
          </h4>
          <div className="c-report-detail__grid">
            {report.resolvedBy && (
              <div className="c-report-detail__field">
                <label>{report.status === "resolved" ? "Resolved by" : "Dismissed by"}</label>
                <HStack spacing={2}>
                  <Avatar user={report.resolvedBy} clickable={false} />
                  <span className="font-medium">{report.resolvedBy.name}</span>
                </HStack>
              </div>
            )}
            {report.resolvedAt && (
              <div className="c-report-detail__field">
                <label>{report.status === "resolved" ? "Resolved at" : "Dismissed at"}</label>
                <span className="c-report-detail__timestamp">
                  <Moment locale={Fider.currentLocale} date={report.resolvedAt} />
                </span>
              </div>
            )}
          </div>
          {report.resolutionNote && (
            <div className="c-report-detail__resolution-note">
              <label>Notes</label>
              <div className="c-report-detail__resolution-note-content">
                {report.resolutionNote}
              </div>
            </div>
          )}
          {!report.resolutionNote && (
            <div className="c-report-detail__no-note">
              No notes provided
            </div>
          )}
        </div>
      )}
    </div>
  )
}

