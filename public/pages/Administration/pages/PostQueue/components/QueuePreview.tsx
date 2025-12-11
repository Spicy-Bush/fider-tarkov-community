import React from "react"
import { Button, Icon, EditOriginalPostPanel } from "@fider/components"
import { classSet } from "@fider/services"
import {
  heroiconsChevronUp as IconChevronUp,
  heroiconsExclamation as IconExclamation,
  heroiconsRefresh as IconRefresh,
} from "@fider/icons.generated"
import { Post, Tag, Comment } from "@fider/models"
import { PostViewer } from "./PostViewer"
import { PostQueueDuplicateSearch } from "./PostQueueDuplicateSearch"

interface QueuePreviewProps {
  selectedPost: Post | null
  tags: Tag[]
  postComments: Comment[]
  postAttachments: string[]
  isCurrentPostTaggedByOther: boolean
  showDuplicateSearch: boolean
  duplicateOriginalNumber: number
  editingOriginalPost: Post | null
  editingOriginalAttachments: string[]
  selectedImagesToTransfer: string[]
  isLoadingOriginalPost: boolean
  editPanelKey: number
  onDeselectPost: () => void
  onRefreshTaggedPost: () => void
  onDismissTaggedPost: () => void
  onShowDuplicateSearch: () => void
  onHideDuplicateSearch: () => void
  onDuplicateSelected: (postNumber: number) => Promise<void>
  onDuplicateReset: () => void
  onPostUpdated: (post: Post) => void
  onContentCopied: () => void
  onOriginalPostSaved: () => void
  onOriginalPostCancelled: () => void
}

export const QueuePreview: React.FC<QueuePreviewProps> = ({
  selectedPost,
  tags,
  postComments,
  postAttachments,
  isCurrentPostTaggedByOther,
  showDuplicateSearch,
  duplicateOriginalNumber,
  editingOriginalPost,
  editingOriginalAttachments,
  selectedImagesToTransfer,
  isLoadingOriginalPost,
  editPanelKey,
  onDeselectPost,
  onRefreshTaggedPost,
  onDismissTaggedPost,
  onShowDuplicateSearch,
  onHideDuplicateSearch,
  onDuplicateSelected,
  onDuplicateReset,
  onPostUpdated,
  onContentCopied,
  onOriginalPostSaved,
  onOriginalPostCancelled,
}) => {
  return (
    <div
      className={classSet({
        "c-queue-split-view__preview": true,
        "c-queue-split-view__preview--mobile-open": selectedPost !== null,
      })}
    >
      {selectedPost && (
        <Button
          variant="tertiary"
          size="small"
          className="c-queue-split-view__mobile-back"
          onClick={onDeselectPost}
        >
          <Icon sprite={IconChevronUp} className="c-queue-split-view__mobile-back-icon" />
          <span>Back to list</span>
        </Button>
      )}
      
      {isCurrentPostTaggedByOther && (
        <div className="c-queue-tagged-banner">
          <div className="c-queue-tagged-banner__text">
            <Icon sprite={IconExclamation} className="h-5 w-5" />
            <span>This post was tagged by another user</span>
          </div>
          <div className="c-queue-tagged-banner__actions">
            <Button variant="tertiary" size="small" onClick={onRefreshTaggedPost}>
              <Icon sprite={IconRefresh} className="h-4" />
              <span>Refresh</span>
            </Button>
            <Button variant="secondary" size="small" onClick={onDismissTaggedPost}>
              Dismiss
            </Button>
          </div>
        </div>
      )}
      
      <PostViewer
        post={selectedPost}
        tags={tags}
        comments={postComments}
        attachments={postAttachments}
        isLoading={false}
        allTags={tags}
        showDuplicateSearch={showDuplicateSearch}
        duplicateOriginalNumber={duplicateOriginalNumber}
        onShowDuplicateSearch={onShowDuplicateSearch}
        onDuplicateSelected={onDuplicateSelected}
        onDuplicateCancelled={onHideDuplicateSearch}
        onDuplicateReset={onDuplicateReset}
        onPostUpdated={onPostUpdated}
        onContentCopied={onContentCopied}
      />
      
      {showDuplicateSearch && selectedPost && (
        <div className="c-duplicate-overlay">
          <PostQueueDuplicateSearch
            excludePostNumber={selectedPost.number}
            tags={tags}
            onSelect={onDuplicateSelected}
            onCancel={onHideDuplicateSearch}
          />
        </div>
      )}
      
      {editingOriginalPost && (
        <EditOriginalPostPanel
          key={editPanelKey}
          post={editingOriginalPost}
          attachments={editingOriginalAttachments}
          imagesToTransfer={selectedImagesToTransfer}
          isLoading={isLoadingOriginalPost}
          onSave={onOriginalPostSaved}
          onCancel={onOriginalPostCancelled}
        />
      )}
    </div>
  )
}

