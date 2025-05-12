import React, { useEffect, useRef, useState } from "react"
import { Comment, Post, ImageUpload, isPostLocked, UserRole } from "@fider/models"
import {
  Reactions,
  Avatar,
  UserName,
  Moment,
  Form,
  Button,
  Markdown,
  Modal,
  ImageViewer,
  MultiImageUploader,
  Dropdown,
  Icon,
  CommentEditor,
} from "@fider/components"
import { HStack } from "@fider/components/layout"
import { formatDate, Failure, actions, notify, copyToClipboard, classSet, clearUrlHash } from "@fider/services"
import { useFider } from "@fider/hooks"
import IconDotsHorizontal from "@fider/assets/images/heroicons-dots-horizontal.svg"
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { useUserStanding } from "@fider/contexts/UserStandingContext"

interface ShowCommentProps {
  post: Post
  comment: Comment
  highlighted?: boolean
  onToggleReaction?: () => void
}

export const ShowComment = (props: ShowCommentProps) => {
  const fider = useFider()
  const { isMuted, muteReason } = useUserStanding()
  const node = useRef<HTMLDivElement | null>(null)
  const [isEditing, setIsEditing] = useState(false)
  const [newContent, setNewContent] = useState<string>(props.comment.content)
  const [isDeleting, setIsDeleting] = useState(false)
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [attachments, setAttachments] = useState<ImageUpload[]>([])
  const [localReactionCounts, setLocalReactionCounts] = useState(props.comment.reactionCounts)
  const emojiSelectorRef = useRef<HTMLDivElement>(null)

  const [error, setError] = useState<Failure>()

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
    if (!fider.session.isAuthenticated) {
      return false
    }

    if (isMuted) {
      return false
    }

    // If user is collaborator or admin, they can edit any comment
    if (fider.session.user.isCollaborator || fider.session.user.isAdministrator) {
      return true
    }

    // If user is moderator, they can only edit comments from regular users
    if (fider.session.user.isModerator) {
      return props.comment.user.role === UserRole.Visitor
    }

    // Regular users can only edit their own comments
    return fider.session.user.id === props.comment.user.id
  }

  const canDeleteComment = () => {
    if (!fider.session.isAuthenticated) {
      return false
    }

    if (isMuted) {
      return false
    }

    // If user is collaborator or admin, they can delete any comment
    if (fider.session.user.isCollaborator || fider.session.user.isAdministrator) {
      return true
    }

    // If user is moderator, they can only delete comments from regular users
    if (fider.session.user.isModerator) {
      return props.comment.user.role === UserRole.Visitor
    }

    // Regular users can only delete their own comments
    return fider.session.user.id === props.comment.user.id
  }

  const clearError = () => setError(undefined)

  const cancelEdit = async () => {
    setIsEditing(false)
    setNewContent(props.comment.content)
    clearError()
  }

  const saveEdit = async () => {
    const response = await actions.updateComment(props.post.number, props.comment.id, newContent, attachments)
    if (response.ok) {
      location.reload()
    } else {
      setError(response.error)
    }
  }

  const handleDelete = async () => {
    setIsDeleting(true)
    setError(undefined)

    const result = await actions.deleteComment(props.post.number, props.comment.id)
    if (result.ok) {
      setShowDeleteModal(false)
      notify.success(t({ id: "action.deletecomment.success", message: "Comment deleted successfully" }))
      // Reload the page to reflect the changes
      window.location.reload()
    } else {
      setError(result.error)
    }
    setIsDeleting(false)
  }

  const toggleReaction = async (emoji: string) => {
    if (isMuted) {
      notify.error(t({ id: "showpost.comment.muted", message: `You are currently muted. Reason: ${muteReason}` }))
      return
    }

    if (isPostLocked(props.post)) {
      notify.error(t({ id: "showpost.comment.locked", message: "This post is locked and cannot be reacted to." }))
      return
    }

    const response = await actions.toggleCommentReaction(props.post.number, props.comment.id, emoji)
    if (response.ok) {
      const added = response.data.added

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
  }

  const onActionSelected = (action: string) => () => {
    if (action === "edit") {
      setIsEditing(true)
    } else if (action === "delete") {
      setShowDeleteModal(true)
    } else if (action === "copylink") {
      const url = `${window.location.origin}${window.location.pathname}#comment-${props.comment.id}`
      copyToClipboard(url)
      notify.success(t({ id: "action.copylink.success", message: "Link copied to clipboard" }))
    }
  }

  const modal = () => {
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
    <span data-tooltip={`This comment has been edited by ${comment.editedBy.name} on ${formatDate(fider.currentLocale, comment.editedAt)}`}>· edited</span>
  )

  const classList = classSet({
    "flex-grow rounded-md p-2": true,
    "bg-gray-100": !props.highlighted,
    "highlighted-comment": props.highlighted,
  })

  return (
    <div id={`comment-${comment.id}`}>
      <HStack spacing={2} className="c-comment flex-items-baseline">
        {modal()}
        <div className="pt-4">
          <Avatar user={comment.user} />
        </div>
        <div ref={node} className={classList}>
          <div className="mb-1">
            <HStack justify="between">
              <HStack>
                <UserName user={comment.user} />{" "}
                <div className="text-xs">
                  · <Moment locale={fider.currentLocale} date={comment.createdAt} /> {editedMetadata}
                </div>
              </HStack>
              {!isEditing && (
                <Dropdown position="left" renderHandle={<Icon sprite={IconDotsHorizontal} width="16" height="16" />}>
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
                      <Dropdown.ListItem onClick={onActionSelected("delete")} className="text-red-700">
                        <Trans id="action.delete">Delete</Trans>
                      </Dropdown.ListItem>
                    </>
                  )}
                </Dropdown>
              )}
            </HStack>
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
                {comment.attachments && comment.attachments.map((x) => <ImageViewer key={x} bkey={x} />)}
                {!isPostLocked(props.post) && !isMuted && (
                  <Reactions reactions={localReactionCounts} emojiSelectorRef={emojiSelectorRef} toggleReaction={toggleReaction} />
                )}
              </>
            )}
          </div>
        </div>
      </HStack>
    </div>
  )
}
