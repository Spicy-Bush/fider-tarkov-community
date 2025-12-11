import { useState, useCallback, useEffect, useRef } from "react"
import { Post, PostStatus } from "@fider/models"
import { actions, Failure, notify } from "@fider/services"

interface UseResponseModalConfig {
  post: Post
  attachments: string[]
  showModal: boolean
  hasCopiedContent?: boolean
}

interface UseResponseModalResult {
  status: string
  setStatus: (status: string) => void
  text: string
  setText: (text: string) => void
  originalNumber: number
  error: Failure | undefined
  setError: (error: Failure | undefined) => void
  showDuplicateSearch: boolean
  setShowDuplicateSearch: (show: boolean) => void
  copiedContent: boolean
  setCopiedContent: (copied: boolean) => void
  showEditOriginal: boolean
  editPanelKey: number
  editingOriginalPost: Post | null
  editingOriginalAttachments: string[]
  isLoadingOriginalPost: boolean
  submit: () => Promise<void>
  handleStatusChange: (value: string) => void
  handleDuplicateSelect: (postNumber: number) => Promise<void>
  handleDuplicateCancel: () => void
  handleCopyContent: () => Promise<void>
  handleSaveOriginal: () => void
  handleCancelEditOriginal: () => void
  loadAndShowEditPanel: (postNumber: number) => Promise<void>
}

export const useResponseModal = (config: UseResponseModalConfig): UseResponseModalResult => {
  const { post, attachments, showModal, hasCopiedContent: initialCopiedContent } = config

  const [status, setStatus] = useState(post.status)
  const [text, setText] = useState(post.response?.text || "")
  const [originalNumber, setOriginalNumber] = useState(0)
  const [error, setError] = useState<Failure | undefined>()
  const [showDuplicateSearch, setShowDuplicateSearch] = useState(false)
  const [copiedContent, setCopiedContent] = useState(false)
  const [showEditOriginal, setShowEditOriginal] = useState(false)
  const [editPanelKey, setEditPanelKey] = useState(0)
  const [editingOriginalPost, setEditingOriginalPost] = useState<Post | null>(null)
  const [editingOriginalAttachments, setEditingOriginalAttachments] = useState<string[]>([])
  const [isLoadingOriginalPost, setIsLoadingOriginalPost] = useState(false)

  const postNumberRef = useRef(post.number)
  const statusRef = useRef(status)
  const showModalRef = useRef(showModal)

  useEffect(() => {
    if (post.number !== postNumberRef.current) {
      postNumberRef.current = post.number
      setStatus(post.status)
      setText(post.response?.text || "")
      setOriginalNumber(0)
      setError(undefined)
    }
  }, [post.number, post.status, post.response?.text])

  useEffect(() => {
    if (showModal) {
      if (initialCopiedContent) {
        setCopiedContent(true)
      }
    } else {
      setShowDuplicateSearch(false)
      setShowEditOriginal(false)
      setCopiedContent(false)
      setEditingOriginalPost(null)
      setEditingOriginalAttachments([])
    }
  }, [showModal, initialCopiedContent])

  useEffect(() => {
    statusRef.current = status
    showModalRef.current = showModal
  }, [status, showModal])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'c' && showModalRef.current && statusRef.current === PostStatus.Duplicate.value) {
        const selection = window.getSelection()
        if (selection && selection.toString().trim()) {
          setCopiedContent(true)
        }
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [])

  const submit = useCallback(async () => {
    const result = await actions.respond(post.number, { status, text, originalNumber })
    if (result.ok) {
      location.reload()
    } else {
      setError(result.error)
    }
  }, [post.number, status, text, originalNumber])

  const handleStatusChange = useCallback((value: string) => {
    setStatus(value)
  }, [])

  const loadAndShowEditPanel = useCallback(async (postNumber: number) => {
    setEditPanelKey(k => k + 1)
    setIsLoadingOriginalPost(true)
    setShowEditOriginal(true)
    
    try {
      const [postResult, attachmentsResult] = await Promise.all([
        actions.getPost(postNumber),
        actions.getPostAttachments(postNumber),
      ])
      if (postResult.ok && postResult.data) {
        setEditingOriginalPost(postResult.data)
        setEditingOriginalAttachments(attachmentsResult.ok ? attachmentsResult.data || [] : [])
      } else {
        setShowEditOriginal(false)
        const errorMessage = postResult.error?.errors?.[0]?.message || "Failed to load the original post"
        notify.error(errorMessage)
      }
    } catch {
      setShowEditOriginal(false)
      notify.error("An unexpected error occurred while loading the original post")
    }
    setIsLoadingOriginalPost(false)
  }, [])

  const handleDuplicateSelect = useCallback(async (postNumber: number) => {
    setOriginalNumber(postNumber)
    setShowDuplicateSearch(false)
    
    const hasContentToCopy = copiedContent || attachments.length > 0
    
    if (hasContentToCopy) {
      await loadAndShowEditPanel(postNumber)
    }
  }, [copiedContent, attachments, loadAndShowEditPanel])

  const handleDuplicateCancel = useCallback(() => {
    setShowDuplicateSearch(false)
    if (originalNumber === 0) {
      setStatus(post.status)
    }
  }, [originalNumber, post.status])

  const handleCopyContent = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(post.description || "")
      setCopiedContent(true)
    } catch {
      notify.error("Failed to copy to clipboard")
    }
  }, [post.description])

  const handleSaveOriginal = useCallback(() => {
    setShowEditOriginal(false)
    setEditingOriginalPost(null)
    setEditingOriginalAttachments([])
    notify.success("Changes saved successfully")
  }, [])

  const handleCancelEditOriginal = useCallback(() => {
    setShowEditOriginal(false)
    setEditingOriginalPost(null)
    setEditingOriginalAttachments([])
  }, [])

  return {
    status,
    setStatus,
    text,
    setText,
    originalNumber,
    error,
    setError,
    showDuplicateSearch,
    setShowDuplicateSearch,
    copiedContent,
    setCopiedContent,
    showEditOriginal,
    editPanelKey,
    editingOriginalPost,
    editingOriginalAttachments,
    isLoadingOriginalPost,
    submit,
    handleStatusChange,
    handleDuplicateSelect,
    handleDuplicateCancel,
    handleCopyContent,
    handleSaveOriginal,
    handleCancelEditOriginal,
    loadAndShowEditPanel,
  }
}

