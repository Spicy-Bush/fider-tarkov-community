import React, { useState } from "react"
import { Post } from "@fider/models"
import * as postActions from "@fider/services/actions/post"
import { Failure } from "@fider/services"
import { default as navigator } from "@fider/services/navigator"
import { Form } from "@fider/components/common/form/Form"
import { Modal } from "@fider/components/common/Modal"
import { Button } from "@fider/components/common/Button"
import { TextArea } from "@fider/components/common/form/TextArea"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"

interface DeletePostModalProps {
  post: Post
  showModal: boolean
  onModalClose: () => void
}

export const DeletePostModal = (props: DeletePostModalProps) => {
  const [text, setText] = useState("")
  const [error, setError] = useState<Failure>()

  const handleDelete = async () => {
    const response = await postActions.deletePost(props.post.number, text)
    if (response.ok) {
      props.onModalClose()
      navigator.goHome()
    } else if (response.error) {
      setError(response.error)
    }
  }

  if (!props.post.permissions.delete) {
    return null
  }

  const modal = (
    <Modal.Window isOpen={props.showModal} onClose={props.onModalClose} center={false} size="large">
      <Modal.Content>
        <Form error={error}>
          <TextArea
            field="text"
            onChange={setText}
            value={text}
            placeholder={i18n._("showpost.moderationpanel.text.placeholder", { message: "Why are you deleting this post? (optional)" })}
          >
            <span className="text-muted">
              <Trans id="showpost.moderationpanel.text.help">
                This operation <strong>cannot</strong> be undone.
              </Trans>
            </span>
          </TextArea>
        </Form>
      </Modal.Content>

      <Modal.Footer>
        <Button variant="danger" onClick={handleDelete}>
          <Trans id="action.delete">Delete</Trans>
        </Button>
        <Button variant="tertiary" onClick={props.onModalClose}>
          <Trans id="action.cancel">Cancel</Trans>
        </Button>
      </Modal.Footer>
    </Modal.Window>
  )

  return modal
}
