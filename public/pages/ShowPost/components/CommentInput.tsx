import React, { useState } from "react"

import { Post, ImageUpload } from "@fider/models"
import { Avatar, UserName, Button, Form, MultiImageUploader } from "@fider/components"
import { SignInModal } from "@fider/components"

import { cache, actions, Failure } from "@fider/services"
import { HStack } from "@fider/components/layout"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"

import { CommentEditor } from "@fider/components"
import { useFider } from "@fider/hooks"

interface CommentInputProps {
  post: Post
}

const CACHE_TITLE_KEY = "CommentInput-Comment-"

export const CommentInput = (props: CommentInputProps) => {
  const getCacheKey = () => `${CACHE_TITLE_KEY}${props.post.id}`

  const getContentFromCache = () => {
    return cache.session.get(getCacheKey())
  }

  const fider = useFider()
  // const inputRef = useRef<HTMLTextAreaElement>()
  const [content, setContent] = useState<string>((fider.session.isAuthenticated && getContentFromCache()) || "")
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const [attachments, setAttachments] = useState<ImageUpload[]>([])
  const [error, setError] = useState<Failure | undefined>(undefined)

  const settings = fider.session.tenant.generalSettings || {
    maxImagesPerComment: 2,
    commentingDisabledFor: [] as string[],
    commentingGloballyDisabled: false
  }
  
  // Updated to safely check user role and handle when user might be undefined
  const isCommentingDisabled = !fider.session.isAuthenticated || 
    (fider.session.user?.role !== "administrator" && 
    (settings.commentingGloballyDisabled || 
    settings.commentingDisabledFor?.includes(fider.session.user?.role || "") || false))

  const hideModal = () => setIsSignInModalOpen(false)
  const clearError = () => setError(undefined)

  const submit = async () => {
    clearError()

    const result = await actions.createComment(props.post.number, content, attachments)
    if (result.ok) {
      cache.session.remove(getCacheKey())
      location.reload()
    } else {
      setError(result.error)
    }
  }

  const editorFocused = () => {
    console.log("focused")
    if (!fider.session.isAuthenticated) {
      setIsSignInModalOpen(true)
    }
  }

  const commentChanged = (newContent: string) => {
    setContent(newContent)
    cache.session.set(getCacheKey(), newContent)
  }

  const hasContent = content?.length > 0

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
      <HStack spacing={2} className="c-comment-input">
        {fider.session.isAuthenticated && <Avatar user={fider.session.user} />}
        <div className="flex-grow bg-gray-50 rounded-md p-2">
          <Form error={error}>
            {isCommentingDisabled && fider.session.isAuthenticated && (
              <div className="c-message c-message--warning">
                <Trans id="showpost.commentinput.disabled">
                  Commenting has been disabled by the administrators.
                </Trans>
              </div>
            )}
            {fider.session.isAuthenticated && (
              <div className="mb-1">
                <UserName user={fider.session.user} />
              </div>
            )}
            <CommentEditor
              initialValue={content}
              onChange={commentChanged}
              onFocus={editorFocused}
              readOnly={isCommentingDisabled}
              placeholder={i18n._("showpost.commentinput.placeholder", { message: "Leave a comment" })}
            />
            {hasContent && !isCommentingDisabled && (
              <>
                <MultiImageUploader field="attachments" maxUploads={settings.maxImagesPerComment || 2} onChange={setAttachments} />
                <Button variant="primary" onClick={submit}>
                  <Trans id="action.submit">Submit</Trans>
                </Button>
              </>
            )}
          </Form>
        </div>
      </HStack>
    </>
  )
}
