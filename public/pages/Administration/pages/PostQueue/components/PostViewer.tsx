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
      <div className="c-queue-detail c-queue-detail--empty">
        <div className="c-queue-detail__empty-state">
          <p className="text-muted">Select a post to view details</p>
        </div>
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="c-queue-detail c-queue-detail--loading">
        <Loader />
      </div>
    )
  }

  return (
    <div className="c-queue-detail">
      <div className="c-queue-detail__section">
        <div className="c-queue-detail__section-header">
          <h4 className="c-queue-detail__section-title">Post #{post.number}</h4>
          <a
            href={`/posts/${post.number}/${post.slug}`}
            target="_blank"
            rel="noopener noreferrer"
            className="c-queue-detail__external-link"
          >
            <Icon sprite={IconExternalLink} className="h-4" />
            <span>View on site</span>
          </a>
        </div>

        <div className="c-queue-detail__author">
          <HStack spacing={2}>
            <Avatar user={post.user} clickable={false} />
            <div>
              <div className="font-medium">{post.user.name}</div>
              <div className="text-xs text-muted">
                <Moment locale={Fider.currentLocale} date={post.createdAt} />
              </div>
            </div>
          </HStack>
        </div>

        <div className="c-queue-detail__content">
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
              <h5 className="c-queue-detail__post-title">{post.title}</h5>
              {post.description ? (
                <div className="c-queue-detail__post-body">
                  <Button
                    variant="tertiary"
                    size="small"
                    className="c-queue-detail__post-body-copy"
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

      <div className="c-queue-detail__section">
        <h4 className="c-queue-detail__section-title">Tags</h4>
        <TagsPanel post={post} tags={tags} onTagsChanged={onTagsChanged} />
      </div>

      <div className="c-queue-detail__section">
        <h4 className="c-queue-detail__section-title">Voting</h4>
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

      <div className="c-queue-detail__section">
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

