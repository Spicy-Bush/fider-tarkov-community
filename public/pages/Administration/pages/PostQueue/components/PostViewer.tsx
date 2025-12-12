import React, { useState, useEffect, useRef } from "react"
import {
  Button,
  Loader,
  Icon,
  Avatar,
  Moment,
  Markdown,
  ImageGallery,
  Input,
  TextArea,
  Form,
  MultiImageUploader,
} from "@fider/components"
import { HStack } from "@fider/components/layout"
import { actions, Fider, Failure } from "@fider/services"
import {
  heroiconsExternalLink as IconExternalLink,
  heroiconsCheck as IconCheck,
  heroiconsX as IconX,
  heroiconsDuplicate as IconCopy,
} from "@fider/icons.generated"
import { Post, Tag, Comment, ImageUpload } from "@fider/models"
import { VoteSection } from "@fider/pages/ShowPost/components/VoteSection"
import { TagsPanel } from "@fider/pages/ShowPost/components/TagsPanel"
import { DiscussionPanel } from "@fider/pages/ShowPost/components/DiscussionPanel"
import { PostQueueActions } from "./PostQueueActions"

interface ClickableUser {
  id: number
  name: string
  avatarURL: string
  role?: string
  status?: string
}

export interface PostViewerProps {
  post: Post | null
  tags: Tag[]
  comments: Comment[]
  attachments: string[]
  isLoading: boolean
  allTags: Tag[]
  showDuplicateSearch: boolean
  duplicateOriginalNumber: number
  onShowDuplicateSearch: () => void
  onDuplicateSelected: (postNumber: number) => void
  onDuplicateCancelled: () => void
  onDuplicateReset: () => void
  onPostUpdated: (post: Post) => void
  onContentCopied: () => void
  onTagsChanged?: (postNumber: number) => void
  onNextPost?: () => void
  onUserClick?: (user: ClickableUser) => void
}

export const PostViewer: React.FC<PostViewerProps> = ({
  post,
  tags,
  comments,
  attachments,
  isLoading,
  allTags,
  showDuplicateSearch,
  duplicateOriginalNumber,
  onShowDuplicateSearch,
  onDuplicateSelected,
  onDuplicateCancelled,
  onDuplicateReset,
  onPostUpdated,
  onContentCopied,
  onTagsChanged,
  onNextPost,
  onUserClick,
}) => {
  const [editMode, setEditMode] = useState(false)
  const [editTitle, setEditTitle] = useState("")
  const [editDescription, setEditDescription] = useState("")
  const [editAttachments, setEditAttachments] = useState<ImageUpload[]>([])
  const [editError, setEditError] = useState<Failure | undefined>()
  const [isSaving, setIsSaving] = useState(false)
  const [copiedField, setCopiedField] = useState<string | null>(null)
  const contentRef = useRef<HTMLDivElement>(null)

  const handleCopyContent = (content: string, field: string) => {
    navigator.clipboard.writeText(content)
    setCopiedField(field)
    onContentCopied()
    setTimeout(() => setCopiedField(null), 2000)
  }

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "c") {
        const selection = window.getSelection()
        if (
          selection &&
          selection.toString().trim() &&
          contentRef.current?.contains(selection.anchorNode)
        ) {
          onContentCopied()
        }
      }
    }
    document.addEventListener("keydown", handleKeyDown)
    return () => document.removeEventListener("keydown", handleKeyDown)
  }, [onContentCopied])

  useEffect(() => {
    if (post) {
      setEditTitle(post.title)
      setEditDescription(post.description || "")
      setEditAttachments(attachments.map((bkey) => ({ bkey, remove: false })))
    }
    setEditMode(false)
  }, [post?.id])

  useEffect(() => {
    setEditAttachments(attachments.map((bkey) => ({ bkey, remove: false })))
  }, [attachments])

  const handleStartEdit = () => {
    if (post) {
      setEditTitle(post.title)
      setEditDescription(post.description || "")
      setEditAttachments(attachments.map((bkey) => ({ bkey, remove: false })))
      setEditError(undefined)
      setEditMode(true)
    }
  }

  const handleCancelEdit = () => {
    setEditMode(false)
    setEditError(undefined)
  }

  const handleSaveEdit = async () => {
    if (!post) return
    setIsSaving(true)
    setEditError(undefined)

    const result = await actions.updatePost(
      post.number,
      editTitle,
      editDescription,
      editAttachments
    )
    if (result.ok) {
      const updatedPost = { ...post, title: editTitle, description: editDescription }
      onPostUpdated(updatedPost)
      setEditMode(false)
    } else {
      setEditError(result.error)
    }
    setIsSaving(false)
  }

  if (!post) {
    return (
      <div className="bg-elevated rounded-panel min-h-[400px] flex items-center justify-center">
        <div className="text-center p-8">
          <p className="text-muted">Select a post to view details</p>
        </div>
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="bg-elevated rounded-panel min-h-[400px] flex items-center justify-center">
        <Loader />
      </div>
    )
  }

  return (
    <div className="bg-elevated rounded-panel min-h-[400px]">
      <div className="p-4 px-5 max-lg:p-3 max-lg:px-4 border-b border-surface-alt last:border-b-0">
        <div className="flex items-center justify-between mb-3">
          <h4 className="text-base font-semibold text-foreground m-0">Post #{post.number}</h4>
          <a
            href={`/posts/${post.number}/${post.slug}`}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-sm text-primary hover:text-primary-hover"
          >
            <Icon sprite={IconExternalLink} className="h-4" />
            <span>View on site</span>
          </a>
        </div>

        <div 
          className="mb-4 p-3 bg-tertiary rounded-card cursor-pointer hover:bg-surface-alt transition-colors group relative"
          onClick={() => onUserClick?.({
            id: post.user.id,
            name: post.user.name,
            avatarURL: post.user.avatarURL,
            role: post.user.role,
            status: post.user.status,
          })}
          role="button"
          tabIndex={0}
        >
          <HStack spacing={2}>
            <Avatar user={post.user} clickable={false} />
            <div>
              <div className="font-medium">{post.user.name}</div>
              <div className="text-xs text-muted">
                <Moment locale={Fider.currentLocale} date={post.createdAt} />
              </div>
            </div>
          </HStack>
          <span className="text-xs text-primary opacity-0 group-hover:opacity-100 transition-opacity absolute right-3 top-1/2 -translate-y-1/2">View profile</span>
        </div>

        <div className="bg-elevated border border-surface-alt rounded-card p-4">
          {editMode ? (
            <Form error={editError}>
              <Input
                field="title"
                label="Title"
                maxLength={100}
                value={editTitle}
                onChange={setEditTitle}
              />
              <TextArea
                field="description"
                label="Description"
                value={editDescription}
                onChange={setEditDescription}
                minRows={5}
              />
              <MultiImageUploader
                field="attachments"
                bkeys={editAttachments
                  .map((a) => a.bkey)
                  .filter((b): b is string => !!b)}
                maxUploads={3}
                onChange={setEditAttachments}
              />
              <HStack spacing={2} className="mt-4">
                <Button
                  variant="primary"
                  size="small"
                  onClick={handleSaveEdit}
                  disabled={isSaving}
                >
                  <Icon sprite={IconCheck} className="h-4" />
                  <span>{isSaving ? "Saving..." : "Save"}</span>
                </Button>
                <Button
                  variant="tertiary"
                  size="small"
                  onClick={handleCancelEdit}
                  disabled={isSaving}
                >
                  <Icon sprite={IconX} className="h-4" />
                  <span>Cancel</span>
                </Button>
              </HStack>
            </Form>
          ) : (
            <div ref={contentRef}>
              <h5 className="text-lg font-semibold text-foreground m-0 mb-3 break-words">{post.title}</h5>
              {post.description ? (
                <div className="text-muted leading-relaxed [&_.c-markdown]:text-sm">
                  <Button
                    variant="tertiary"
                    size="small"
                    className="float-right ml-2 mb-1"
                    onClick={() =>
                      handleCopyContent(post.description || "", "description")
                    }
                  >
                    <Icon
                      sprite={copiedField === "description" ? IconCheck : IconCopy}
                      className="h-4"
                    />
                    <span className="text-xs">
                      {copiedField === "description" ? "Copied" : "Copy"}
                    </span>
                  </Button>
                  <Markdown text={post.description} style="full" />
                </div>
              ) : (
                <em className="text-muted">No description provided.</em>
              )}
              {attachments.length > 0 && (
                <div className="mt-4">
                  <ImageGallery bkeys={attachments} />
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <div className="p-4 px-5 max-lg:p-3 max-lg:px-4 border-b border-surface-alt last:border-b-0">
        <h4 className="text-base font-semibold text-foreground m-0 mb-3">Tags</h4>
        <TagsPanel post={post} tags={tags} onTagsChanged={onTagsChanged} onNextPost={onNextPost} />
      </div>

      <div className="p-4 px-5 max-lg:p-3 max-lg:px-4 border-b border-surface-alt last:border-b-0">
        <h4 className="text-base font-semibold text-foreground m-0 mb-3">Voting</h4>
        <VoteSection post={post} />
      </div>

      <PostQueueActions
        post={post}
        tags={allTags}
        showDuplicateSearch={showDuplicateSearch}
        duplicateOriginalNumber={duplicateOriginalNumber}
        onShowDuplicateSearch={onShowDuplicateSearch}
        onDuplicateSelected={onDuplicateSelected}
        onDuplicateCancelled={onDuplicateCancelled}
        onDuplicateReset={onDuplicateReset}
        onEditPost={handleStartEdit}
        isEditMode={editMode}
      />

      <div className="p-4 px-5 max-lg:p-3 max-lg:px-4 border-b border-surface-alt last:border-b-0">
        <DiscussionPanel
          post={post}
          comments={comments}
          subscribed={false}
          reportedCommentIds={[]}
          dailyLimitReached={false}
        />
      </div>
    </div>
  )
}
