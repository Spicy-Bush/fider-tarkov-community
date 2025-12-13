import React, { useMemo, useState, useCallback } from "react"
import { Button, Icon, EditOriginalPostPanel } from "@fider/components"
import { UserProfile } from "@fider/components/UserProfile"
import { classSet, Fider } from "@fider/services"
import {
  heroiconsChevronUp as IconChevronUp,
  heroiconsExclamation as IconExclamation,
  heroiconsRefresh as IconRefresh,
  heroiconsEye as IconEye,
  heroiconsArrowLeft as IconArrowLeft,
} from "@fider/icons.generated"
import { Post, Tag, Comment, ViewerInfo, User } from "@fider/models"
import { PostViewer } from "./PostViewer"
import { PostQueueDuplicateSearch } from "./PostQueueDuplicateSearch"

interface ViewingUserType {
  id: number
  name: string
  avatarURL: string
  role?: string
  status?: string
}

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
  viewers: ViewerInfo[]
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
  onNextPost: () => void
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
  viewers,
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
  onNextPost,
}) => {
  const [viewingUser, setViewingUser] = useState<ViewingUserType | null>(null)
  const [profileKey, setProfileKey] = useState(0)

  const otherViewers = useMemo(() => 
    viewers.filter((v) => v.userId !== Fider.session.user.id),
    [viewers]
  )

  const handleUserClick = useCallback((user: ViewingUserType) => {
    setViewingUser(user)
    setProfileKey(prev => prev + 1)
  }, [])

  const handleCloseUserProfile = useCallback(() => {
    setViewingUser(null)
  }, [])

  const handleUserUpdate = useCallback((updates: any) => {
    if (viewingUser) {
      setViewingUser({ ...viewingUser, ...updates })
    }
  }, [viewingUser])

  return (
    <div
      id="queue-preview-container"
      className={classSet({
        "flex-1 min-w-0 h-full min-h-full overflow-y-auto bg-tertiary rounded-panel border border-surface-alt relative lg:max-h-[90vh]": true,
        "max-lg:hidden": selectedPost === null && viewingUser === null,
        "max-lg:fixed max-lg:inset-0 max-lg:z-modal max-lg:overflow-y-auto max-lg:p-4": selectedPost !== null || viewingUser !== null,
      })}
    >
      {viewingUser ? (
        <div className="p-4">
          <Button
            variant="tertiary"
            size="small"
            className="flex mb-3"
            onClick={handleCloseUserProfile}
          >
            <Icon sprite={IconArrowLeft} className="h-4" />
            <span>Back to post</span>
          </Button>
          <UserProfile
            key={profileKey}
            userId={viewingUser.id}
            user={viewingUser as User}
            embedded
            compact
            onUserUpdate={handleUserUpdate}
          >
            <UserProfile.Header compact />
            <UserProfile.Actions />
            <UserProfile.Status />
            <UserProfile.Tabs>
              <UserProfile.Search />
              <UserProfile.Standing />
            </UserProfile.Tabs>
          </UserProfile>
        </div>
      ) : (
        <>
          {selectedPost && (
            <div className="hidden max-lg:flex justify-between items-center mb-3">
              <Button
                variant="tertiary"
                size="small"
                className="flex"
                onClick={onDeselectPost}
              >
                <Icon sprite={IconChevronUp} className="-rotate-90 w-4 h-4" />
                <span>Back to list</span>
              </Button>
              {otherViewers.length > 0 && (
                <span
                  className="inline-flex items-center gap-1 px-2 py-1 bg-info-medium text-primary rounded-full text-sm font-medium cursor-help"
                  data-tooltip={otherViewers.map((v) => v.userName).join(", ")}
                >
                  <Icon sprite={IconEye} className="h-4" />
                  <span>{otherViewers.length}</span>
                </span>
              )}
            </div>
          )}
          
          {isCurrentPostTaggedByOther && (
            <div className="flex items-center justify-between gap-2 p-3 px-4 bg-danger-light border-b border-danger-light text-danger text-sm mb-3 rounded-card">
              <div className="flex items-center gap-2 [&_svg]:text-danger [&_svg]:shrink-0">
                <Icon sprite={IconExclamation} className="h-5 w-5" />
                <span>This post was tagged by another user</span>
              </div>
              <div className="flex items-center gap-2">
                <Button variant="tertiary" size="small" onClick={onRefreshTaggedPost}>
                  <Icon sprite={IconRefresh} className="h-4" />
                  <span>Refresh</span>
                </Button>
                <Button variant="tertiary" size="small" onClick={onDismissTaggedPost}>
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
            onNextPost={onNextPost}
            onUserClick={handleUserClick}
          />
      
      {showDuplicateSearch && selectedPost && (
            <div className="absolute inset-0 z-50 bg-elevated overflow-y-auto rounded-panel animate-[fadeIn_0.15s_ease] max-lg:fixed max-lg:rounded-none">
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
        </>
      )}
    </div>
  )
}
