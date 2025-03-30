import React, { useState } from "react"
import { Post } from "@fider/models"
import { Modal, Button, TextArea } from "@fider/components"
import { actions } from "@fider/services"
import { Trans } from "@lingui/react/macro"

interface PostLockingModalProps {
  post: Post
  isOpen: boolean
  onClose: () => void
  mode: "lock" | "unlock"
}

export const PostLockingModal = (props: PostLockingModalProps) => {
  const [message, setMessage] = useState("")
  const isLockMode = props.mode === "lock"

  const submit = async () => {
    const response = isLockMode 
      ? await actions.lockPost(props.post.number, message)
      : await actions.unlockPost(props.post.number)
    
    if (response.ok) {
      location.reload()
    }
  }

  return (
    <Modal.Window isOpen={props.isOpen} onClose={props.onClose}>
      <Modal.Header>
        {isLockMode ? (
          <Trans id="modal.lockpost.header">Lock Post</Trans>
        ) : (
          <Trans id="modal.unlockpost.header">Unlock Post</Trans>
        )}
      </Modal.Header>
      <Modal.Content>
        <p>
          {isLockMode ? (
            <Trans id="modal.lockpost.text">
              Locking this post will prevent users from commenting, voting, or editing it. 
              Only administrators and collaborators can unlock a locked post.
            </Trans>
          ) : (
            <Trans id="modal.unlockpost.text">
              Unlocking this post will allow users to comment, vote, and edit it again. 
              Are you sure you want to unlock this post?
            </Trans>
          )}
        </p>
        {isLockMode && (
          <TextArea
            field="message"
            placeholder="Optional message explaining why this post is locked"
            value={message}
            onChange={setMessage}
          />
        )}
      </Modal.Content>
      <Modal.Footer>
        <Button 
          variant={isLockMode ? "danger" : "primary"} 
          onClick={submit}
        >
          {isLockMode ? (
            <Trans id="action.lock">Lock</Trans>
          ) : (
            <Trans id="action.unlock">Unlock</Trans>
          )}
        </Button>
        <Button variant="tertiary" onClick={props.onClose}>
          <Trans id="action.cancel">Cancel</Trans>
        </Button>
      </Modal.Footer>
    </Modal.Window>
  )
}