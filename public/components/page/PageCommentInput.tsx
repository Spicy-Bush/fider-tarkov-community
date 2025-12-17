import React, { useState } from "react"
import { Comment, Page } from "@fider/models"
import { Avatar, UserName, Button, Form, SignInModal, CommentEditor } from "@fider/components"
import { Failure } from "@fider/services"
import { addPageComment } from "@fider/services/pages"
import { useFider } from "@fider/hooks"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"

interface PageCommentInputProps {
  page: Page
  onCommentAdded?: (comment: Comment) => void
}

export const PageCommentInput = (props: PageCommentInputProps) => {
  const fider = useFider()
  const [content, setContent] = useState("")
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  const [error, setError] = useState<Failure | undefined>(undefined)
  const [editorKey, setEditorKey] = useState(0)

  const hideModal = () => setIsSignInModalOpen(false)
  const clearError = () => setError(undefined)

  const submit = async () => {
    clearError()

    const result = await addPageComment(props.page.id, content)
    if (result.ok) {
      if (props.onCommentAdded) {
        props.onCommentAdded(result.data)
        setContent("")
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

  const hasContent = content?.length > 0

  if (!fider.session.isAuthenticated) {
    return <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
  }

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
      <div className="mt-3 rounded-card p-3 bg-tertiary">
        <Form error={error}>
          <div className="flex items-center gap-2 mb-2">
            <Avatar user={fider.session.user} size="small" />
            <UserName user={fider.session.user} />
          </div>
          <CommentEditor
            key={editorKey}
            initialValue={content}
            onChange={setContent}
            onFocus={editorFocused}
            placeholder={i18n._("showpost.commentinput.placeholder", { message: "Leave a comment" })}
          />
          {hasContent && (
            <div className="mt-2">
              <Button variant="primary" onClick={submit}>
                <Trans id="action.submit">Submit</Trans>
              </Button>
            </div>
          )}
        </Form>
      </div>
    </>
  )
}

