import React from "react"
import { Button, Loader, Icon, Avatar, Moment, Markdown } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { Fider, classSet } from "@fider/services"
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
      <div className="bg-elevated rounded-panel min-h-[400px] flex items-center justify-center">
        <div className="text-center p-8">
          <p className="text-muted">Select a report to view details</p>
        </div>
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="bg-elevated rounded-panel min-h-[400px] flex items-center justify-center">
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
    <div className="bg-elevated rounded-panel min-h-[400px]">
      <div className="p-4 px-5 max-lg:p-3 max-lg:px-4 border-b border-surface-alt bg-tertiary">
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
            className={classSet({
              "px-3 py-1 rounded-full text-sm font-medium": true,
              "bg-success-medium text-success": report.status === "resolved",
              "bg-surface-alt text-muted": report.status === "dismissed",
            })}
          >
            {report.status === "resolved" ? "Resolved" : "Dismissed"}
          </span>
        )}
      </div>

      <div className="p-4 px-5 max-lg:p-3 max-lg:px-4 border-b border-surface-alt">
        <h4 className="text-base font-semibold text-foreground m-0 mb-3">Report Information</h4>
        <div className="grid grid-cols-2 gap-4 max-lg:grid-cols-1">
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted uppercase tracking-wide">Reason</label>
            <span className="inline-block px-2 py-1 rounded bg-warning/10 text-warning text-sm font-medium w-fit">{report.reason}</span>
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted uppercase tracking-wide">Reported by</label>
            <div
              className="p-3 bg-tertiary rounded-card cursor-pointer hover:bg-surface-alt transition-colors group relative"
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
              <span className="text-xs text-primary opacity-0 group-hover:opacity-100 transition-opacity absolute right-3 top-1/2 -translate-y-1/2">View profile</span>
            </div>
          </div>
        </div>
        {report.details && (
          <div className="mt-4">
            <label className="text-xs text-muted uppercase tracking-wide block mb-1">Additional Details</label>
            <div className="p-3 bg-tertiary rounded-card text-sm">{report.details}</div>
          </div>
        )}
      </div>

      <div className="p-4 px-5 max-lg:p-3 max-lg:px-4 border-b border-surface-alt">
        <div className="flex items-center justify-between mb-3">
          <h4 className="text-base font-semibold text-foreground m-0">
            Reported {report.reportedType === "post" ? "Post" : "Comment"}
          </h4>
          <a
            href={getTargetLink()}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-sm text-primary hover:text-primary-hover"
          >
            <Icon sprite={IconExternalLink} className="h-4" />
            <span>View original</span>
          </a>
        </div>

        {reportedUser && (
          <div
            className="p-3 mb-4 bg-tertiary rounded-card cursor-pointer hover:bg-surface-alt transition-colors group relative"
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
            <span className="text-xs text-primary opacity-0 group-hover:opacity-100 transition-opacity absolute right-3 top-1/2 -translate-y-1/2">View profile</span>
          </div>
        )}

        <div className="bg-elevated border border-surface-alt rounded-card p-4">
          {report.reportedType === "post" && post && (
            <>
              <h5 className="text-lg font-semibold text-foreground m-0 mb-3 break-words">{post.title}</h5>
              {post.description && (
                <div className="text-muted leading-relaxed [&_.c-markdown]:text-sm">
                  <Markdown text={post.description} style="full" />
                </div>
              )}
            </>
          )}

          {report.reportedType === "comment" && comment && (
            <>
              <div className="text-muted leading-relaxed [&_.c-markdown]:text-sm">
                <Markdown text={comment.content} style="full" />
              </div>
              {post && (
                <div className="mt-3 pt-3 border-t border-surface-alt text-sm text-muted wrap-anywhere">
                  On post:{" "}
                  <a
                    href={`/posts/${post.number}/${post.slug}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:text-primary-hover"
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
        <div className="p-4 px-5 max-lg:p-3 max-lg:px-4 border-b border-surface-alt bg-info-light">
          <h4 className="text-base font-semibold text-foreground m-0 mb-3">Assignment</h4>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-muted uppercase tracking-wide">Assigned to</label>
            <HStack spacing={2}>
              <Avatar user={report.assignedTo} clickable={false} />
              <span className="font-medium">{report.assignedTo?.name}</span>
            </HStack>
          </div>
        </div>
      )}

      {(report.status === "resolved" || report.status === "dismissed") && (
        <div className={classSet({
          "p-4 px-5 max-lg:p-3 max-lg:px-4 border-b border-surface-alt last:border-b-0": true,
          "bg-success-light": report.status === "resolved",
          "bg-surface-alt": report.status === "dismissed",
        })}>
          <h4 className="text-base font-semibold text-foreground m-0 mb-3">
            {report.status === "resolved" ? "Resolution Details" : "Dismissal Details"}
          </h4>
          <div className="grid grid-cols-2 gap-4 max-lg:grid-cols-1">
            {report.resolvedBy && (
              <div className="flex flex-col gap-1">
                <label className="text-xs text-muted uppercase tracking-wide">{report.status === "resolved" ? "Resolved by" : "Dismissed by"}</label>
                <HStack spacing={2}>
                  <Avatar user={report.resolvedBy} clickable={false} />
                  <span className="font-medium">{report.resolvedBy.name}</span>
                </HStack>
              </div>
            )}
            {report.resolvedAt && (
              <div className="flex flex-col gap-1">
                <label className="text-xs text-muted uppercase tracking-wide">{report.status === "resolved" ? "Resolved at" : "Dismissed at"}</label>
                <span className="text-sm">
                  <Moment locale={Fider.currentLocale} date={report.resolvedAt} />
                </span>
              </div>
            )}
          </div>
          {report.resolutionNote && (
            <div className="mt-4">
              <label className="text-xs text-muted uppercase tracking-wide block mb-1">Notes</label>
              <div className="p-3 bg-elevated rounded-card text-sm">
                {report.resolutionNote}
              </div>
            </div>
          )}
          {!report.resolutionNote && (
            <div className="mt-4 text-sm text-muted italic">
              No notes provided
            </div>
          )}
        </div>
      )}
    </div>
  )
}
