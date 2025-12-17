import { useCallback } from "react"
import { Post, ImageUpload, PostStatus } from "@fider/models"
import { actions, notify, Failure, postPermissions, Fider } from "@fider/services"
import React from "react"
import { Trans } from "@lingui/react/macro"

interface UseShowPostActionsConfig {
  post: Post
  newTitle: string
  newDescription: string
  attachments: ImageUpload[]
  setError: (error: Failure | undefined) => void
  startEdit: () => void
  openModal: (name: "delete" | "response" | "report" | "lock" | "unlock") => void
}

interface UseShowPostActionsResult {
  saveChanges: () => Promise<void>
  canEdit: boolean
  canDelete: boolean
  canRespond: boolean
  canLock: boolean
  canArchive: boolean
  onActionSelected: (action: "copy" | "delete" | "status" | "edit" | "lock" | "unlock" | "report" | "archive" | "unarchive") => () => Promise<void>
}

export const useShowPostActions = (config: UseShowPostActionsConfig): UseShowPostActionsResult => {
  const { post, newTitle, newDescription, attachments, setError, startEdit, openModal } = config

  const saveChanges = useCallback(async () => {
    const result = await actions.updatePost(post.number, newTitle, newDescription, attachments)
    if (result.ok) {
      location.reload()
    } else {
      setError(result.error)
    }
  }, [post.number, newTitle, newDescription, attachments, setError])

  const canEdit = postPermissions.canEdit(post)
  const canDelete = (() => {
    const status = PostStatus.Get(post.status)
    if (status.closed) return false
    return postPermissions.canDelete(post)
  })()
  const canRespond = postPermissions.canRespondAny()
  const canLock = postPermissions.canLock()
  const canArchive = postPermissions.canArchive()

  const onActionSelected = useCallback(
    (action: "copy" | "delete" | "status" | "edit" | "lock" | "unlock" | "report" | "archive" | "unarchive") => async () => {
      if (action === "copy") {
        navigator.clipboard.writeText(window.location.href)
        notify.success(<Trans id="showpost.copylink.success">Link copied to clipboard</Trans>)
      } else if (action === "delete") {
        openModal("delete")
      } else if (action === "status") {
        openModal("response")
      } else if (action === "edit") {
        startEdit()
      } else if (action === "lock") {
        openModal("lock")
      } else if (action === "unlock") {
        openModal("unlock")
      } else if (action === "report") {
        openModal("report")
      } else if (action === "archive") {
        const result = await actions.archivePost(post.number)
        if (result.ok) {
          notify.success(<Trans id="showpost.archive.success">Post has been archived</Trans>)
          location.reload()
        }
      } else if (action === "unarchive") {
        const result = await actions.unarchivePost(post.number)
        if (result.ok) {
          notify.success(<Trans id="showpost.unarchive.success">Post has been unarchived</Trans>)
          location.reload()
        }
      }
    },
    [startEdit, openModal, post.number]
  )

  return {
    saveChanges,
    canEdit,
    canDelete,
    canRespond,
    canLock,
    canArchive,
    onActionSelected,
  }
}

