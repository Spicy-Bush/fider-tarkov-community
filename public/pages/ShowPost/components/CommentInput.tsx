import React, { useState } from "react"

import { Post, ImageUpload, isPostLocked, isPostArchived, Comment } from "@fider/models"
import { Avatar, UserName, Button, Form, MultiImageUploader } from "@fider/components"
import { SignInModal } from "@fider/components"

import { cache, actions, Failure } from "@fider/services"
import { HStack } from "@fider/components/layout"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"

import { CommentEditor } from "@fider/components"
import { useFider } from "@fider/hooks"
import { useUserStanding } from "@fider/contexts/UserStandingContext"

interface CommentInputProps {
  post: Post
  onCommentAdded?: (comment: Comment) => void
}

const CACHE_TITLE_KEY = "CommentInput-Comment-"

export const CommentInput = (props: CommentInputProps) => {
  const getCacheKey = () => `${CACHE_TITLE_KEY}${props.post.id}`

  const getContentFromCache = () => {
    return cache.session.get(getCacheKey())
  }

  const fider = useFider()
  const { isMuted, muteReason } = useUserStanding()
  const [content, setContent] = useState<string>((fider.session.isAuthenticated && getContentFromCache()) || "")
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const [attachments, setAttachments] = useState<ImageUpload[]>([])
  const [error, setError] = useState<Failure | undefined>(undefined)
  const [editorKey, setEditorKey] = useState(0)

  const settings = fider.session.tenant.generalSettings || {
    maxImagesPerComment: 2,
    commentingDisabledFor: [] as string[],
    commentingGloballyDisabled: false
  }
  
  // TODO: refactor the mess that is this check logic
  const isCommentingDisabled = !fider.session.isAuthenticated || 
    isPostLocked(props.post) || 
    isMuted ||
    (fider.session.user?.role !== "administrator" && 
    (settings.commentingGloballyDisabled || 
    settings.commentingDisabledFor?.includes(fider.session.user?.role || "") || false))

  const hideModal = () => setIsSignInModalOpen(false)
  const clearError = () => setError(undefined)

  const submit = async () => {
    clearError()
    const wasArchived = isPostArchived(props.post)

    const result = await actions.createComment(props.post.number, content, attachments)
    if (result.ok) {
      cache.session.remove(getCacheKey())
      
      if (wasArchived) {
        location.reload()
        return
      }
      
      if (props.onCommentAdded && fider.session.user) {
        const newComment: Comment = {
          id: result.data.id,
          content: content,
          createdAt: new Date().toISOString(),
          user: fider.session.user,
          attachments: result.data.attachments || [],
        }
        props.onCommentAdded(newComment)
        setContent("")
        setAttachments([])
        setEditorKey(prev => prev + 1)
      } else {
        location.reload()
      }
    } else {
      setError(result.error)
    }
  }

  const editorFocused = () => {
    if (!fider.session.isAuthenticated) {
      setIsSignInModalOpen(true)
    }
  }

  const commentChanged = (newContent: string) => {
    setContent(newContent)
    cache.session.set(getCacheKey(), newContent)
  }

  const hasContent = content?.length > 0

  if (!fider.session.isAuthenticated) {
    return <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
  }

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
      <div className="mt-3 rounded-card p-3 bg-tertiary">
        <Form error={error}>
          {isCommentingDisabled && (
            <div className="p-3 rounded bg-warning-light border border-warning-light text-warning text-sm">
              {isMuted ? (
                <Trans id="showpost.commentinput.muted">
                  You are currently muted. Reason: {muteReason}
                </Trans>
              ) : (
                <Trans id="showpost.commentinput.disabled">
                  Commenting has been disabled by the administrators.
                </Trans>
              )}
            </div>
          )}
          {!isCommentingDisabled && (
            <>
              <div className="flex items-center gap-2 mb-2">
                <Avatar user={fider.session.user} size="small" />
                <UserName user={fider.session.user} />
              </div>
              <CommentEditor
                key={editorKey}
                initialValue={content}
                onChange={commentChanged}
                onFocus={editorFocused}
                readOnly={isCommentingDisabled}
                placeholder={i18n._("showpost.commentinput.placeholder", { message: "Leave a comment" })}
              />
              {hasContent && !isCommentingDisabled && (
                <div className="mt-2">
                  <MultiImageUploader field="attachments" maxUploads={settings.maxImagesPerComment || 2} onChange={setAttachments} />
                  <Button variant="primary" onClick={submit}>
                    <Trans id="action.submit">Submit</Trans>
                  </Button>
                </div>
              )}
            </>
          )}
        </Form>
      </div>
    </>
  )
}
