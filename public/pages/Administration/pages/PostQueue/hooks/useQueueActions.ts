import { useState, useCallback } from "react"
import { Post } from "@fider/models"
import { actions } from "@fider/services"

interface UseQueueActionsConfig {
  selectedPost: Post | null
  selectPost: (post: Post) => void
  deselectPost: () => void
  removeTaggedPostFromList: (post: Post | null) => void
  loadPostDetails: (postNumber: number, refreshPost?: boolean) => Promise<void>
  setTaggedByOtherIds: React.Dispatch<React.SetStateAction<Set<number>>>
  setPosts: React.Dispatch<React.SetStateAction<Post[]>>
  loadPosts: () => Promise<void>
  setNewPostIds: React.Dispatch<React.SetStateAction<Set<number>>>
  postAttachments: string[]
  setShowDuplicateSearch: React.Dispatch<React.SetStateAction<boolean>>
  pushState: (state: Record<string, unknown>) => void
  isNavigating: React.MutableRefObject<boolean>
}

interface UseQueueActionsResult {
  duplicateOriginalNumber: number
  copiedFromPostId: number | null
  selectedImagesToTransfer: string[]
  pendingDuplicateNumber: number
  editingOriginalPost: Post | null
  editingOriginalAttachments: string[]
  isLoadingOriginalPost: boolean
  editPanelKey: number
  handleSelectPost: (post: Post) => void
  handleDeselectPost: () => void
  handleShowDuplicateSearch: () => void
  handleHideDuplicateSearch: () => void
  handleContentCopied: () => void
  handleDuplicateSelected: (postNumber: number) => Promise<void>
  handleOriginalPostSaved: () => void
  handleOriginalPostCancelled: () => void
  handleDuplicateReset: () => void
  handleRefresh: () => void
  handleRefreshTaggedPost: () => Promise<void>
  handleDismissTaggedPost: () => void
}

export const useQueueActions = (config: UseQueueActionsConfig): UseQueueActionsResult => {
  const {
    selectedPost,
    selectPost,
    deselectPost,
    removeTaggedPostFromList,
    loadPostDetails,
    setTaggedByOtherIds,
    setPosts,
    loadPosts,
    setNewPostIds,
    postAttachments,
    setShowDuplicateSearch,
    pushState,
    isNavigating,
  } = config

  const [duplicateOriginalNumber, setDuplicateOriginalNumber] = useState(0)
  const [copiedFromPostId, setCopiedFromPostId] = useState<number | null>(null)
  const [selectedImagesToTransfer, setSelectedImagesToTransfer] = useState<string[]>([])
  const [pendingDuplicateNumber, setPendingDuplicateNumber] = useState(0)
  const [editingOriginalPost, setEditingOriginalPost] = useState<Post | null>(null)
  const [editingOriginalAttachments, setEditingOriginalAttachments] = useState<string[]>([])
  const [isLoadingOriginalPost, setIsLoadingOriginalPost] = useState(false)
  const [editPanelKey, setEditPanelKey] = useState(0)

  const handleSelectPost = useCallback((post: Post) => {
    removeTaggedPostFromList(selectedPost)
    if (!isNavigating.current) {
      pushState({ postId: post.id })
    }
    selectPost(post)
    setCopiedFromPostId(null)
    setSelectedImagesToTransfer([])
  }, [selectedPost, selectPost, removeTaggedPostFromList, pushState, isNavigating])

  const handleDeselectPost = useCallback(() => {
    removeTaggedPostFromList(selectedPost)
    if (!isNavigating.current) {
      pushState({})
    }
    deselectPost()
  }, [selectedPost, deselectPost, removeTaggedPostFromList, pushState, isNavigating])

  const handleShowDuplicateSearch = useCallback(() => {
    if (!isNavigating.current && selectedPost) {
      pushState({ postId: selectedPost.id, duplicate: true })
    }
    setShowDuplicateSearch(true)
  }, [selectedPost, pushState, isNavigating])

  const handleHideDuplicateSearch = useCallback(() => {
    if (!isNavigating.current && selectedPost) {
      pushState({ postId: selectedPost.id })
    }
    setShowDuplicateSearch(false)
  }, [selectedPost, pushState, isNavigating])

  const handleContentCopied = useCallback(() => {
    if (selectedPost) {
      setCopiedFromPostId(selectedPost.id)
    }
  }, [selectedPost])

  const handleDuplicateSelected = useCallback(async (postNumber: number) => {
    const hasContentToCopy = copiedFromPostId === selectedPost?.id || postAttachments.length > 0

    if (hasContentToCopy) {
      setPendingDuplicateNumber(postNumber)
      setSelectedImagesToTransfer(postAttachments)
      setEditPanelKey(k => k + 1)
      setIsLoadingOriginalPost(true)
      
      const [postResult, attachmentsResult] = await Promise.all([
        actions.getPost(postNumber),
        actions.getPostAttachments(postNumber),
      ])
      if (postResult.ok && postResult.data) {
        setEditingOriginalPost(postResult.data)
        setEditingOriginalAttachments(attachmentsResult.ok ? attachmentsResult.data || [] : [])
      }
      setIsLoadingOriginalPost(false)
    } else {
      setDuplicateOriginalNumber(postNumber)
    }
  }, [copiedFromPostId, selectedPost, postAttachments, handleHideDuplicateSearch])

  const handleOriginalPostSaved = useCallback(() => {
    setEditingOriginalPost(null)
    setEditingOriginalAttachments([])
    setSelectedImagesToTransfer([])
    setDuplicateOriginalNumber(pendingDuplicateNumber)
    setPendingDuplicateNumber(0)
  }, [pendingDuplicateNumber])

  const handleOriginalPostCancelled = useCallback(() => {
    setEditingOriginalPost(null)
    setEditingOriginalAttachments([])
    setDuplicateOriginalNumber(pendingDuplicateNumber)
    setPendingDuplicateNumber(0)
  }, [pendingDuplicateNumber])

  const handleDuplicateReset = useCallback(() => {
    setDuplicateOriginalNumber(0)
  }, [])

  const handleRefresh = useCallback(() => {
    setNewPostIds(new Set())
    setTaggedByOtherIds(new Set())
    loadPosts()
  }, [setNewPostIds, setTaggedByOtherIds, loadPosts])

  const handleRefreshTaggedPost = useCallback(async () => {
    if (selectedPost) {
      setTaggedByOtherIds((prev) => {
        const newSet = new Set(prev)
        newSet.delete(selectedPost.id)
        return newSet
      })
      await loadPostDetails(selectedPost.number, true)
    }
  }, [selectedPost, setTaggedByOtherIds, loadPostDetails])

  const handleDismissTaggedPost = useCallback(() => {
    if (selectedPost) {
      setTaggedByOtherIds((prev) => {
        const newSet = new Set(prev)
        newSet.delete(selectedPost.id)
        return newSet
      })
      setPosts((prev) => prev.filter((p) => p.id !== selectedPost.id))
      deselectPost()
    }
  }, [selectedPost, setTaggedByOtherIds, setPosts, deselectPost])

  return {
    duplicateOriginalNumber,
    copiedFromPostId,
    selectedImagesToTransfer,
    pendingDuplicateNumber,
    editingOriginalPost,
    editingOriginalAttachments,
    isLoadingOriginalPost,
    editPanelKey,
    handleSelectPost,
    handleDeselectPost,
    handleShowDuplicateSearch,
    handleHideDuplicateSearch,
    handleContentCopied,
    handleDuplicateSelected,
    handleOriginalPostSaved,
    handleOriginalPostCancelled,
    handleDuplicateReset,
    handleRefresh,
    handleRefreshTaggedPost,
    handleDismissTaggedPost,
  }
}

