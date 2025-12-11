import React, { useState } from "react"
import { Button, Modal, Select, TextArea, Form, SelectOption } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { Post, PostStatus, Tag } from "@fider/models"
import { actions, Failure } from "@fider/services"
import { useFider } from "@fider/hooks"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"

interface PostQueueActionsProps {
  post: Post
  tags: Tag[]
  showDuplicateSearch: boolean
  duplicateOriginalNumber: number
  onShowDuplicateSearch: () => void
  onDuplicateSelected: (postNumber: number) => void
  onDuplicateCancelled: () => void
  onDuplicateReset: () => void
  onEditPost: () => void
  isEditMode?: boolean
}

export const PostQueueActions: React.FC<PostQueueActionsProps> = ({ 
  post, 
  tags,
  showDuplicateSearch,
  duplicateOriginalNumber,
  onShowDuplicateSearch,
  onDuplicateSelected,
  onDuplicateCancelled,
  onDuplicateReset,
  onEditPost,
  isEditMode,
}) => {
  const fider = useFider()
  const [showStatusModal, setShowStatusModal] = useState(false)
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [showLockModal, setShowLockModal] = useState(false)
  const [status, setStatus] = useState(post.status)
  const [responseText, setResponseText] = useState(post.response?.text || "")
  const [deleteText, setDeleteText] = useState("")
  const [lockMessage, setLockMessage] = useState("")
  const [error, setError] = useState<Failure | undefined>()
  const prevDuplicateNumberRef = React.useRef(duplicateOriginalNumber)

  const canChangeStatus = fider.session.user.isCollaborator || fider.session.user.isModerator || fider.session.user.isAdministrator
  const canLock = fider.session.user.isCollaborator || fider.session.user.isAdministrator
  const canDelete = fider.session.user.isCollaborator || fider.session.user.isAdministrator || fider.session.user.isModerator

  const handleStatusChange = (opt?: SelectOption) => {
    if (opt) {
      setStatus(opt.value)
      if (opt.value === PostStatus.Duplicate.value) {
        setShowStatusModal(false)
        onShowDuplicateSearch()
      }
    }
  }

  React.useEffect(() => {
    if (
      duplicateOriginalNumber > 0 && 
      prevDuplicateNumberRef.current !== duplicateOriginalNumber
    ) {
      setStatus(PostStatus.Duplicate.value)
      setShowStatusModal(true)
    }
    prevDuplicateNumberRef.current = duplicateOriginalNumber
  }, [duplicateOriginalNumber])

  const handleStatusSubmit = async () => {
    setError(undefined)
    const result = await actions.respond(post.number, {
      status,
      text: responseText,
      originalNumber: duplicateOriginalNumber,
    })
    if (result.ok) {
      location.reload()
    } else {
      setError(result.error)
    }
  }

  const handleDeleteSubmit = async () => {
    setError(undefined)
    const result = await actions.deletePost(post.number, deleteText)
    if (result.ok) {
      location.reload()
    } else {
      setError(result.error)
    }
  }

  const handleLockSubmit = async () => {
    setError(undefined)
    const result = post.lockedAt
      ? await actions.unlockPost(post.number)
      : await actions.lockPost(post.number, lockMessage)
    if (result.ok) {
      location.reload()
    } else {
      setError(result.error)
    }
  }

  const statusOptions = PostStatus.All.map((s) => {
    const id = `enum.poststatus.${s.value.toString()}`
    return {
      value: s.value.toString(),
      label: i18n._(id, { message: s.title }),
    }
  })

  const handleDecline = () => {
    setStatus(PostStatus.Declined.value)
    setShowStatusModal(true)
  }

  const handleDuplicate = () => {
    setStatus(PostStatus.Duplicate.value)
    onShowDuplicateSearch()
  }

  return (
    <div className="c-queue-detail__section c-queue-detail__section--actions">
      <h4 className="c-queue-detail__section-title">Actions</h4>
      <HStack justify="between" className="flex-wrap">
        <HStack spacing={2}>
          {canChangeStatus && (
            <Button size="small" variant="secondary" onClick={handleDuplicate}>
              <Trans id="action.duplicate">Duplicate</Trans>
            </Button>
          )}
          {canChangeStatus && (
          <Button size="small" variant="secondary" onClick={onEditPost} disabled={isEditMode}>
            <Trans id="action.edit">Edit</Trans>
          </Button>
          )}
          {canChangeStatus && (
            <Button size="small" variant="secondary" onClick={() => setShowStatusModal(true)}>
              <Trans id="action.changestatus">Change Status</Trans>
            </Button>
          )}
          {canChangeStatus && (
            <Button size="small" variant="secondary" onClick={handleDecline}>
              <Trans id="action.decline">Decline</Trans>
            </Button>
          )}
        </HStack>
        <HStack spacing={2}>
          {canLock && (
            <Button size="small" variant="secondary" onClick={() => setShowLockModal(true)}>
              {post.lockedAt ? (
                <Trans id="action.unlock">Unlock</Trans>
              ) : (
                <Trans id="action.lock">Lock</Trans>
              )}
            </Button>
          )}
          {canDelete && (
            <Button size="small" variant="danger" onClick={() => setShowDeleteModal(true)}>
              <Trans id="action.delete">Delete</Trans>
            </Button>
          )}
        </HStack>
      </HStack>

      <Modal.Window isOpen={showStatusModal} onClose={() => { setShowStatusModal(false); onDuplicateReset(); }} center={false} size="large">
        <Modal.Header>
          <Trans id="action.changestatus">Change Status</Trans>
        </Modal.Header>
        <Modal.Content>
          <Form error={error}>
            <Select
              field="status"
              label={i18n._("label.status", { message: "Status" })}
              defaultValue={status}
              options={statusOptions}
              onChange={handleStatusChange}
            />
            {status === PostStatus.Duplicate.value ? (
              <>
                {duplicateOriginalNumber > 0 ? (
                  <div className="p-3 bg-gray-100 rounded mb-2">
                    <span className="font-medium">Original Post: </span>
                    <span>#{duplicateOriginalNumber}</span>
                    <Button
                      variant="tertiary"
                      size="small"
                      className="ml-2"
                      onClick={() => {
                        setShowStatusModal(false)
                        onShowDuplicateSearch()
                      }}
                    >
                      Change
                    </Button>
                  </div>
                ) : (
                  <Button
                    variant="secondary"
                    onClick={() => {
                      setShowStatusModal(false)
                      onShowDuplicateSearch()
                    }}
                  >
                    Select Original Post
                  </Button>
                )}
                <TextArea
                  field="text"
                  label={i18n._("label.response", { message: "Response (optional)" })}
                  value={responseText}
                  onChange={setResponseText}
                  minRows={2}
                  placeholder={i18n._("showpost.duplicateresponse.placeholder", {
                    message: "Explain why this is a duplicate...",
                  })}
                />
                <span className="text-muted text-sm">
                  <Trans id="showpost.responseform.message.mergedvotes">
                    Votes from this post will be merged into original post.
                  </Trans>
                </span>
              </>
            ) : (
              <TextArea
                field="text"
                label={i18n._("label.response", { message: "Response" })}
                value={responseText}
                onChange={setResponseText}
                minRows={3}
                placeholder={i18n._("showpost.responseform.text.placeholder", {
                  message: "What's going on with this post? Let your users know what are your plans...",
                })}
              />
            )}
          </Form>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="primary" onClick={handleStatusSubmit}>
            <Trans id="action.submit">Submit</Trans>
          </Button>
          <Button variant="tertiary" onClick={() => setShowStatusModal(false)}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>

      <Modal.Window isOpen={showDeleteModal} onClose={() => setShowDeleteModal(false)} manageHistory={false}>
        <Modal.Header>
          <Trans id="queue.delete.title">Delete Post</Trans>
        </Modal.Header>
        <Modal.Content>
          <Form error={error}>
            <p>
              <Trans id="queue.delete.message">
                Are you sure you want to delete this post? This action cannot be undone.
              </Trans>
            </p>
            <TextArea
              field="text"
              label={i18n._("label.reason", { message: "Reason (optional)" })}
              value={deleteText}
              onChange={setDeleteText}
              minRows={2}
            />
          </Form>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="danger" onClick={handleDeleteSubmit}>
            <Trans id="action.delete">Delete</Trans>
          </Button>
          <Button variant="tertiary" onClick={() => setShowDeleteModal(false)}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>

      <Modal.Window isOpen={showLockModal} onClose={() => setShowLockModal(false)} manageHistory={false}>
        <Modal.Header>
          {post.lockedAt ? (
            <Trans id="modal.unlockpost.header">Unlock Post</Trans>
          ) : (
            <Trans id="modal.lockpost.header">Lock Post</Trans>
          )}
        </Modal.Header>
        <Modal.Content>
          <Form error={error}>
            {post.lockedAt ? (
              <p>
                <Trans id="modal.unlockpost.text">
                  Unlocking this post will allow users to comment, vote, and edit it again.
                </Trans>
              </p>
            ) : (
              <>
                <p>
                  <Trans id="modal.lockpost.text">
                    Locking this post will prevent users from commenting, voting, or editing it.
                  </Trans>
                </p>
                <TextArea
                  field="message"
                  label={i18n._("label.lockmessage", { message: "Lock message (optional)" })}
                  value={lockMessage}
                  onChange={setLockMessage}
                  minRows={2}
                />
              </>
            )}
          </Form>
        </Modal.Content>
        <Modal.Footer>
          <Button variant={post.lockedAt ? "primary" : "danger"} onClick={handleLockSubmit}>
            {post.lockedAt ? (
              <Trans id="action.unlock">Unlock</Trans>
            ) : (
              <Trans id="action.lock">Lock</Trans>
            )}
          </Button>
          <Button variant="tertiary" onClick={() => setShowLockModal(false)}>
            <Trans id="action.cancel">Cancel</Trans>
          </Button>
        </Modal.Footer>
      </Modal.Window>

    </div>
  )
}

