import { useState, useCallback } from "react"
import { DraftImage } from "@fider/services/draftImages"
import { Failure } from "@fider/services"
import { useModalState } from "@fider/hooks/useModalState"

type ShowPostModal = "delete" | "response" | "report" | "lock" | "unlock"

interface UseShowPostStateConfig {
  initialTitle: string
  initialDescription: string
  initialAttachments: string[]
}

interface UseShowPostStateResult {
  editMode: boolean
  setEditMode: (mode: boolean) => void
  newTitle: string
  setNewTitle: (title: string) => void
  newDescription: string
  setNewDescription: (description: string) => void
  attachments: DraftImage[]
  setAttachments: (attachments: DraftImage[]) => void
  error: Failure | undefined
  setError: (error: Failure | undefined) => void
  hasCopiedContent: boolean
  setHasCopiedContent: (copied: boolean) => void
  activeModal: ShowPostModal | null
  isModalOpen: (name: ShowPostModal) => boolean
  openModal: (name: ShowPostModal) => void
  closeModal: () => void
  startEdit: () => void
  cancelEdit: () => void
}

export const useShowPostState = (config: UseShowPostStateConfig): UseShowPostStateResult => {
  const { initialTitle, initialDescription } = config

  const [editMode, setEditMode] = useState(false)
  const [newTitle, setNewTitle] = useState(initialTitle)
  const [newDescription, setNewDescription] = useState(initialDescription)
  const [attachments, setAttachments] = useState<DraftImage[]>(config.initialAttachments.map(bkey => ({ kind: "stored", bkey })))
  const [error, setError] = useState<Failure | undefined>()
  const [hasCopiedContent, setHasCopiedContent] = useState(false)

  const { activeModal, isOpen, open, close } = useModalState<ShowPostModal>()

  const startEdit = useCallback(() => {
    setEditMode(true)
  }, [])

  const cancelEdit = useCallback(() => {
    setError(undefined)
    setEditMode(false)
  }, [])

  return {
    editMode,
    setEditMode,
    newTitle,
    setNewTitle,
    newDescription,
    setNewDescription,
    attachments,
    setAttachments,
    error,
    setError,
    hasCopiedContent,
    setHasCopiedContent,
    activeModal,
    isModalOpen: isOpen,
    openModal: open,
    closeModal: close,
    startEdit,
    cancelEdit,
  }
}
