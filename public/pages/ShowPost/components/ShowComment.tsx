import React, { useEffect, useRef, useState } from "react"
import { Comment, Post, ImageUpload, isPostLocked, isCommentHidden, ReportReason } from "@fider/models"
import {
  Reactions,
  Avatar,
  UserName,
  Moment,
  Form,
  Button,
  Markdown,
  Modal,
  ImageGallery,
  MultiImageUploader,
  Dropdown,
  Icon,
  CommentEditor,
  ReportModal,
  ReportButton,
} from "@fider/components"
import { formatDate, Failure, actions, notify, copyToClipboard, classSet, clearUrlHash, commentPermissions, postPermissions } from "@fider/services"
import { useFider } from "@fider/hooks"
import { heroiconsDotsHorizontal as IconDotsHorizontal } from "@fider/icons.generated"
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { useUserStanding } from "@fider/contexts/UserStandingContext"

interface ShowCommentProps {
  post?: Post
  comment: Comment
  highlighted?: boolean
  onToggleReaction?: () => void
  hasReported?: boolean
  dailyLimitReached?: boolean
  reportReasons?: ReportReason[]
  customToggleReaction?: (emoji: string) => Promise<{ added: boolean } | undefined>
}

export const ShowComment = (props: ShowCommentProps) => {
  const fider = useFider()
  const { isMuted, muteReason } = useUserStanding()
  const node = useRef<HTMLDivElement | null>(null)
  const [isEditing, setIsEditing] = useState(false)
  const [newContent, setNewContent] = useState<string>(props.comment.content)
  const [isDeleting, setIsDeleting] = useState(false)
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [showReportModal, setShowReportModal] = useState(false)
  const [attachments, setAttachments] = useState<ImageUpload[]>([])
  const [localReactionCounts, setLocalReactionCounts] = useState(props.comment.reactionCounts)
  const emojiSelectorRef = useRef<HTMLDivElement>(null)

  const [error, setError] = useState<Failure>()

  const isPostContext = !!props.post

  const handleClick = (e: MouseEvent) => {
    if (node.current == null || !node.current.contains(e.target as Node)) {
      clearUrlHash()
    }
  }

  useEffect(() => {
    if (props.highlighted) {
      document.addEventListener("mousedown", handleClick)
      return () => {
        document.removeEventListener("mousedown", handleClick)
      }
    }
  }, [props.highlighted])

  const canEditComment = () => {
    if (!isPostContext) return false
    if (isMuted) return false
    return commentPermissions.canEdit(props.comment)
  }

  const canDeleteComment = () => {
    if (!isPostContext) return false
    if (isMuted) return false
    return commentPermissions.canDelete(props.comment)
  }

  const clearError = () => setError(undefined)

  const cancelEdit = async () => {
    setIsEditing(false)
    setNewContent(props.comment.content)
    clearError()
  }

  const saveEdit = async () => {
    if (!props.post) return
    const response = await actions.updateComment(props.post.number, props.comment.id, newContent, attachments)
    if (response.ok) {
      location.reload()
    } else {
      setError(response.error)
    }
  }

  const handleDelete = async () => {
    if (!props.post) return
    setIsDeleting(true)
    setError(undefined)

    const result = await actions.deleteComment(props.post.number, props.comment.id)
    if (result.ok) {
      setShowDeleteModal(false)
      notify.success(t({ id: "action.deletecomment.success", message: "Comment deleted successfully" }))
      window.location.reload()
    } else {
      setError(result.error)
    }
    setIsDeleting(false)
  }

  const updateLocalReactions = (emoji: string, added: boolean) => {
    setLocalReactionCounts((prevCounts) => {
      const newCounts = [...(prevCounts ?? [])]
      const reactionIndex = newCounts.findIndex((r) => r.emoji === emoji)
      if (reactionIndex !== -1) {
        const newCount = added ? newCounts[reactionIndex].count + 1 : newCounts[reactionIndex].count - 1
        if (newCount === 0) {
          newCounts.splice(reactionIndex, 1)
        } else {
          newCounts[reactionIndex] = {
            ...newCounts[reactionIndex],
            count: newCount,
            includesMe: added,
          }
        }
      } else if (added) {
        newCounts.push({ emoji, count: 1, includesMe: true })
      }
      return newCounts
    })
  }

  const toggleReaction = async (emoji: string) => {
    if (isMuted) {
      notify.error(t({ id: "showpost.comment.muted", message: `You are currently muted. Reason: ${muteReason}` }))
      return
    }

    if (props.customToggleReaction) {
      const result = await props.customToggleReaction(emoji)
      if (result) {
        updateLocalReactions(emoji, result.added)
      }
      return
    }

    if (!props.post) return

    if (isPostLocked(props.post)) {
      notify.error(t({ id: "showpost.comment.locked", message: "This post is locked and cannot be reacted to." }))
      return
    }

    const response = await actions.toggleCommentReaction(props.post.number, props.comment.id, emoji)
    if (response.ok) {
      updateLocalReactions(emoji, response.data.added)
    }
  }

  const onActionSelected = (action: string) => async () => {
    if (action === "edit") {
      setIsEditing(true)
    } else if (action === "delete") {
      setShowDeleteModal(true)
    } else if (action === "copylink") {
      const url = `${window.location.origin}${window.location.pathname}#comment-${props.comment.id}`
      copyToClipboard(url)
      notify.success(t({ id: "action.copylink.success", message: "Link copied to clipboard" }))
    } else if (action === "report") {
      setShowReportModal(true)
    } else if (action === "hide") {
      const result = await actions.hideComment(props.comment.id)
      if (result.ok) {
        notify.success(t({ id: "showcomment.hide.success", message: "Comment has been hidden" }))
        location.reload()
      }
    } else if (action === "unhide") {
      const result = await actions.unhideComment(props.comment.id)
      if (result.ok) {
        notify.success(t({ id: "showcomment.unhide.success", message: "Comment has been unhidden" }))
        location.reload()
      }
    }
  }

  const modal = () => {
    if (!props.post) return null
    return (
      <Modal.Window isOpen={showDeleteModal} onClose={() => setShowDeleteModal(false)}>
        <Modal.Header>
          <Trans id="action.deletecomment.title">Delete Comment</Trans>
        </Modal.Header>
        <Modal.Content>
          <p>
            <Trans id="action.deletecomment.message">Are you sure you want to delete this comment? This action cannot be undone.</Trans>
          </p>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="tertiary" onClick={() => setShowDeleteModal(false)}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
          <Button variant="danger" onClick={handleDelete} disabled={isDeleting}>
            {isDeleting ? (
              <Trans id="action.deleting">Deleting...</Trans>
            ) : (
              <Trans id="action.delete">Delete</Trans>
            )}
          </Button>
        </Modal.Footer>
      </Modal.Window>
    )
  }

  const comment = props.comment

  const editedMetadata = !!comment.editedAt && !!comment.editedBy && (
    <span 
      className="text-muted whitespace-nowrap"
      data-tooltip={`Edited by ${comment.editedBy.name} on ${formatDate(fider.currentLocale, comment.editedAt)}`}
    >
      · edited
    </span>
  )

  const classList = classSet({
    "rounded-card p-3": true,
    "bg-tertiary": !props.highlighted && !isCommentHidden(props.comment),
    "highlighted-comment": props.highlighted,
    "bg-danger-light border border-danger-medium": isCommentHidden(props.comment),
  })

  const canReact = props.customToggleReaction || (props.post && !isPostLocked(props.post) && !isMuted)
  const showActions = isPostContext

  return (
    <div id={`comment-${comment.id}`} className="mt-3">
      {modal()}
      {isPostContext && props.post && (
        <ReportModal
          isOpen={showReportModal}
          onClose={() => setShowReportModal(false)}
          postNumber={props.post.number}
          commentId={comment.id}
          reasons={props.reportReasons}
        />
      )}
      <div ref={node} className={classList}>
        {isCommentHidden(props.comment) && (
          <div className="text-xs font-medium text-danger mb-2">
            <Trans id="showcomment.hidden.label">Hidden from public view</Trans>
          </div>
        )}
        <div className="flex items-start justify-between gap-2 mb-2">
          <div className="flex items-center gap-2 min-w-0 flex-wrap">
            <Avatar user={comment.user} size="small" />
            <UserName user={comment.user} />
            <span className="text-xs text-muted flex items-center gap-1 flex-wrap">
              <Moment locale={fider.currentLocale} date={comment.createdAt} />
              {editedMetadata}
            </span>
          </div>
          {!isEditing && showActions && (
            <div className="flex items-center gap-1 shrink-0">
              <ReportButton
                reportedUserId={comment.user.id}
                size="small"
                hasReported={props.hasReported || false}
                dailyLimitReached={props.dailyLimitReached || false}
                onReport={() => setShowReportModal(true)}
              />
              <Dropdown position="left" renderHandle={<Icon sprite={IconDotsHorizontal} width="16" height="16" className="cursor-pointer" />}>
                <Dropdown.ListItem onClick={onActionSelected("copylink")}>
                  <Trans id="action.copylink">Copy link</Trans>
                </Dropdown.ListItem>
                {canEditComment() && (
                  <>
                    <Dropdown.Divider />
                    <Dropdown.ListItem onClick={onActionSelected("edit")}>
                      <Trans id="action.edit">Edit</Trans>
                    </Dropdown.ListItem>
                  </>
                )}
                {canDeleteComment() && (
                  <>
                    <Dropdown.Divider />
                    <Dropdown.ListItem onClick={onActionSelected("delete")} className="text-danger">
                      <Trans id="action.delete">Delete</Trans>
                    </Dropdown.ListItem>
                  </>
                )}
                {postPermissions.canHide() && (
                  <>
                    <Dropdown.Divider />
                    {!isCommentHidden(props.comment) ? (
                      <Dropdown.ListItem onClick={onActionSelected("hide")}>
                        <Trans id="action.hide">Hide</Trans>
                      </Dropdown.ListItem>
                    ) : (
                      <Dropdown.ListItem onClick={onActionSelected("unhide")}>
                        <Trans id="action.unhide">Unhide</Trans>
                      </Dropdown.ListItem>
                    )}
                  </>
                )}
              </Dropdown>
            </div>
          )}
        </div>
        <div>
          {isEditing ? (
            <Form error={error}>
              <CommentEditor initialValue={newContent} onChange={setNewContent} placeholder={comment.content} />
              <MultiImageUploader field="attachments" bkeys={comment.attachments} maxUploads={2} onChange={setAttachments} />
              <Button size="small" onClick={saveEdit} variant="primary">
                <Trans id="action.save">Save</Trans>
              </Button>
              <Button variant="tertiary" size="small" onClick={cancelEdit}>
                <Trans id="action.cancel">Cancel</Trans>
              </Button>
            </Form>
          ) : (
            <>
              <Markdown text={comment.content} style="full" />
              {comment.attachments && comment.attachments.length > 0 && <ImageGallery bkeys={comment.attachments} />}
              {canReact && (
                <Reactions reactions={localReactionCounts} emojiSelectorRef={emojiSelectorRef} toggleReaction={toggleReaction} />
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
