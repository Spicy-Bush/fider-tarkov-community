import { useState, useCallback, useRef, useEffect } from "react"
import { Post, Comment } from "@fider/models"
import { actions, PAGINATION } from "@fider/services"
import { useFider } from "@fider/hooks"

export type QueueSortOption = "newest" | "oldest" | "most-wanted" | "least-wanted"

interface UseQueueStateResult {
  posts: Post[]
  setPosts: React.Dispatch<React.SetStateAction<Post[]>>
  total: number
  setTotal: React.Dispatch<React.SetStateAction<number>>
  page: number
  setPage: React.Dispatch<React.SetStateAction<number>>
  perPage: number
  isLoading: boolean
  selectedPost: Post | null
  selectedPostRef: React.MutableRefObject<Post | null>
  postComments: Comment[]
  postAttachments: string[]
  newPostIds: Set<number>
  setNewPostIds: React.Dispatch<React.SetStateAction<Set<number>>>
  taggedByOtherIds: Set<number>
  setTaggedByOtherIds: React.Dispatch<React.SetStateAction<Set<number>>>
  postsRef: React.MutableRefObject<Post[]>
  currentUserIdRef: React.MutableRefObject<number>
  sortOption: QueueSortOption
  setSortOption: React.Dispatch<React.SetStateAction<QueueSortOption>>
  showDuplicateSearch: boolean
  setShowDuplicateSearch: React.Dispatch<React.SetStateAction<boolean>>
  selectPost: (post: Post) => void
  deselectPost: () => void
  loadPosts: () => Promise<void>
  loadPostDetails: (postNumber: number, refreshPost?: boolean) => Promise<void>
  updatePost: (updatedPost: Post) => void
  removeTaggedPostFromList: (post: Post | null) => void
}

export const useQueueState = (): UseQueueStateResult => {
  const fider = useFider()
  const perPage = PAGINATION.QUEUE_LIMIT
  const user = fider.session.user
  const isHelperOnly = user.isHelper && !user.isCollaborator && !user.isModerator && !user.isAdministrator

  const [posts, setPosts] = useState<Post[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [isLoading, setIsLoading] = useState(true)
  const [selectedPost, setSelectedPost] = useState<Post | null>(null)
  const [postComments, setPostComments] = useState<Comment[]>([])
  const [postAttachments, setPostAttachments] = useState<string[]>([])
  const [newPostIds, setNewPostIds] = useState<Set<number>>(new Set())
  const [taggedByOtherIds, setTaggedByOtherIds] = useState<Set<number>>(new Set())
  const [sortOption, setSortOption] = useState<QueueSortOption>("newest")
  const [showDuplicateSearch, setShowDuplicateSearch] = useState(false)

  const selectedPostRef = useRef<Post | null>(null)
  const currentUserIdRef = useRef(user.id)
  const postsRef = useRef<Post[]>([])

  useEffect(() => {
    selectedPostRef.current = selectedPost
  }, [selectedPost])

  useEffect(() => {
    postsRef.current = posts
  }, [posts])

  const loadPosts = useCallback(async () => {
    setIsLoading(true)
    const result = await actions.searchPosts({
      statuses: ["open", "planned", "started", "completed"],
      tags: ["untagged"],
      limit: perPage,
      offset: (page - 1) * perPage,
      view: sortOption,
      date: isHelperOnly ? "7d" : undefined,
      includeCount: true,
    })
    if (result.ok) {
      setPosts(result.data || [])
      const totalCount = result.headers?.get("X-Total-Count")
      if (totalCount) {
        setTotal(parseInt(totalCount, 10))
      } else {
        setTotal(result.data?.length === perPage ? page * perPage + 1 : page * perPage)
      }
    }
    setIsLoading(false)
  }, [page, perPage, sortOption, isHelperOnly])

  const loadPostDetails = useCallback(async (postNumber: number, refreshPost = false) => {
    const [commentsResult, attachmentsResult, postResult] = await Promise.all([
      actions.getAllComments(postNumber),
      actions.getPostAttachments(postNumber),
      refreshPost ? actions.getPost(postNumber) : Promise.resolve(null),
    ])
    if (selectedPostRef.current?.number !== postNumber) return

    if (commentsResult.ok) {
      setPostComments(commentsResult.data || [])
    }
    if (attachmentsResult.ok) {
      setPostAttachments(attachmentsResult.data || [])
    }
    if (postResult?.ok && postResult.data) {
      setSelectedPost(postResult.data)
      setPosts((prev) => prev.map((p) => (p.id === postResult.data!.id ? postResult.data! : p)))
    }
  }, [])

  const removeTaggedPostFromList = useCallback((post: Post | null) => {
    if (post && post.tags && post.tags.length > 0) {
      setPosts((prev) => prev.filter((p) => p.id !== post.id))
    }
  }, [])

  const selectPost = useCallback((post: Post) => {
    setSelectedPost(post)
    setNewPostIds((prev) => {
      const newSet = new Set(prev)
      newSet.delete(post.id)
      return newSet
    })
  }, [])

  const deselectPost = useCallback(() => {
    setSelectedPost(null)
  }, [])

  const updatePost = useCallback((updatedPost: Post) => {
    setSelectedPost(updatedPost)
    setPosts((prev) => prev.map((p) => (p.id === updatedPost.id ? updatedPost : p)))
  }, [])

  return {
    posts,
    setPosts,
    total,
    setTotal,
    page,
    setPage,
    perPage,
    isLoading,
    selectedPost,
    selectedPostRef,
    postComments,
    postAttachments,
    newPostIds,
    setNewPostIds,
    taggedByOtherIds,
    setTaggedByOtherIds,
    postsRef,
    currentUserIdRef,
    sortOption,
    setSortOption,
    showDuplicateSearch,
    setShowDuplicateSearch,
    selectPost,
    deselectPost,
    loadPosts,
    loadPostDetails,
    updatePost,
    removeTaggedPostFromList,
  }
}

