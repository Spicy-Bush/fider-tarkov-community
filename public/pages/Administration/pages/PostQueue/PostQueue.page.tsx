import React, { useEffect, useCallback } from "react"
import { Tag, ViewerInfo } from "@fider/models"
import { PageConfig } from "@fider/components/layouts"
import { useStackNavigation, useRealtimePresence } from "@fider/hooks"
import { useQueueState, useQueueEvents, useQueueActions, QueueSortOption } from "./hooks"
import { QueueList, QueuePreview } from "./components"

import "../PostQueue.page.scss"

export const pageConfig: PageConfig = {
  title: "Post Queue",
  subtitle: "Review and tag untagged posts",
  sidebarItem: "queue",
  layoutVariant: "fullWidth",
}

interface QueueViewers {
  postId: number
  viewers: ViewerInfo[]
}

interface PostQueuePageProps {
  tags: Tag[]
  viewers: QueueViewers[]
}

interface HistoryState extends Record<string, unknown> {
  postId?: number
  duplicate?: boolean
}

const PostQueuePage: React.FC<PostQueuePageProps> = (props) => {
  const state = useQueueState()

  const { viewers, viewItem, stopViewing } = useRealtimePresence({
    eventPrefix: "queue",
    itemIdField: "postId",
    initialViewers: props.viewers,
  })

  const handleStateChange = useCallback(
    (historyState: HistoryState | null) => {
      if (historyState?.postId) {
        if (state.selectedPostRef.current?.id !== historyState.postId) {
          const post = state.posts.find((p) => p.id === historyState.postId)
          if (post) {
            state.selectPost(post)
          } else {
            state.deselectPost()
          }
        }
        state.setShowDuplicateSearch(!!historyState.duplicate)
      } else {
        state.deselectPost()
        state.setShowDuplicateSearch(false)
      }
    },
    [state.posts, state.selectedPostRef, state.selectPost, state.deselectPost, state.setShowDuplicateSearch]
  )

  const { pushState, isNavigating } = useStackNavigation<HistoryState>({
    onStateChange: handleStateChange,
    urlPath: "/admin/queue",
  })

  const actions = useQueueActions({
    selectedPost: state.selectedPost,
    selectPost: state.selectPost,
    deselectPost: state.deselectPost,
    removeTaggedPostFromList: state.removeTaggedPostFromList,
    loadPostDetails: state.loadPostDetails,
    setTaggedByOtherIds: state.setTaggedByOtherIds,
    setPosts: state.setPosts,
    loadPosts: state.loadPosts,
    setNewPostIds: state.setNewPostIds,
    postAttachments: state.postAttachments,
    setShowDuplicateSearch: state.setShowDuplicateSearch,
    pushState,
    isNavigating,
  })

  useQueueEvents({
    postsRef: state.postsRef,
    selectedPostRef: state.selectedPostRef,
    currentUserIdRef: state.currentUserIdRef,
    setPosts: state.setPosts,
    setTotal: state.setTotal,
    setNewPostIds: state.setNewPostIds,
    setTaggedByOtherIds: state.setTaggedByOtherIds,
  })

  useEffect(() => {
    state.loadPosts()
  }, [state.loadPosts])

  const handleSortChange = useCallback((newSort: QueueSortOption) => {
    state.setPage(1)
    state.setSortOption(newSort)
  }, [state.setPage, state.setSortOption])

  useEffect(() => {
    if (state.selectedPost) {
      state.loadPostDetails(state.selectedPost.number)
      viewItem(state.selectedPost.id)
    } else {
      stopViewing()
    }
  }, [state.selectedPost?.id])

  useEffect(() => {
    return () => stopViewing()
  }, [stopViewing])

  const isCurrentPostTaggedByOther = state.selectedPost 
    ? state.taggedByOtherIds.has(state.selectedPost.id) 
    : false

  const currentPostViewers = state.selectedPost
    ? viewers.get(state.selectedPost.id) || []
    : []

  const handleNextPost = useCallback(() => {
    if (!state.selectedPost || state.posts.length === 0) return
    
    const currentIndex = state.posts.findIndex((p) => p.id === state.selectedPost?.id)
    const nextIndex = currentIndex + 1
    
    if (nextIndex < state.posts.length) {
      actions.handleSelectPost(state.posts[nextIndex])
    } else if (state.posts.length > 0) {
      actions.handleSelectPost(state.posts[0])
    }
    
    const previewEl = document.querySelector(".c-queue-split-view__preview--mobile-open")
    if (previewEl) {
      previewEl.scrollTo({ top: 0, behavior: "smooth" })
    } else {
      window.scrollTo({ top: 0, behavior: "smooth" })
    }
  }, [state.selectedPost, state.posts, actions.handleSelectPost])

  return (
    <div className="c-queue-split-view">
      <QueueList
        posts={state.posts}
        total={state.total}
        page={state.page}
        perPage={state.perPage}
        isLoading={state.isLoading}
        selectedPost={state.selectedPost}
        newPostIds={state.newPostIds}
        taggedByOtherIds={state.taggedByOtherIds}
        viewers={viewers}
        sortOption={state.sortOption}
        onSelectPost={actions.handleSelectPost}
        onRefresh={actions.handleRefresh}
        onPrevPage={() => state.setPage((p) => p - 1)}
        onNextPage={() => state.setPage((p) => p + 1)}
        onSortChange={handleSortChange}
      />
      <QueuePreview
        selectedPost={state.selectedPost}
        tags={props.tags}
        postComments={state.postComments}
        postAttachments={state.postAttachments}
        isCurrentPostTaggedByOther={isCurrentPostTaggedByOther}
        showDuplicateSearch={state.showDuplicateSearch}
        duplicateOriginalNumber={actions.duplicateOriginalNumber}
        editingOriginalPost={actions.editingOriginalPost}
        editingOriginalAttachments={actions.editingOriginalAttachments}
        selectedImagesToTransfer={actions.selectedImagesToTransfer}
        isLoadingOriginalPost={actions.isLoadingOriginalPost}
        editPanelKey={actions.editPanelKey}
        viewers={currentPostViewers}
        onDeselectPost={actions.handleDeselectPost}
        onRefreshTaggedPost={actions.handleRefreshTaggedPost}
        onDismissTaggedPost={actions.handleDismissTaggedPost}
        onShowDuplicateSearch={actions.handleShowDuplicateSearch}
        onHideDuplicateSearch={actions.handleHideDuplicateSearch}
        onDuplicateSelected={actions.handleDuplicateSelected}
        onDuplicateReset={actions.handleDuplicateReset}
        onPostUpdated={state.updatePost}
        onContentCopied={actions.handleContentCopied}
        onOriginalPostSaved={actions.handleOriginalPostSaved}
        onOriginalPostCancelled={actions.handleOriginalPostCancelled}
        onNextPost={handleNextPost}
      />
    </div>
  )
}

export default PostQueuePage
