import React from "react"
import { Button, Form, Modal, TextArea } from "@fider/components"
import { Failure } from "@fider/services"
import { i18n } from "@lingui/core"
import { Trans } from "@lingui/react/macro"

interface ResolveModalProps {
  isOpen: boolean
  resolveAction: "resolved" | "dismissed"
  resolutionNote: string
  error: Failure | undefined
  onResolutionNoteChange: (note: string) => void
  onSubmit: () => void
  onClose: () => void
}

export const ResolveModal: React.FC<ResolveModalProps> = ({
  isOpen,
  resolveAction,
  resolutionNote,
  error,
  onResolutionNoteChange,
  onSubmit,
  onClose,
}) => {
  return (
    <Modal.Window isOpen={isOpen} onClose={onClose} center={true} size="small" manageHistory={false}>
      <Modal.Header>
        {resolveAction === "resolved" ? (
          <Trans id="reports.resolve.title">Resolve Report</Trans>
        ) : (
          <Trans id="reports.dismiss.title">Dismiss Report</Trans>
        )}
      </Modal.Header>
      <Modal.Content>
        <Form error={error}>
          <TextArea
            field="resolutionNote"
            label={i18n._("reports.resolution.label", { message: "Resolution Note (optional)" })}
            value={resolutionNote}
            onChange={onResolutionNoteChange}
            placeholder={i18n._("reports.resolution.placeholder", {
              message: "Add any notes about how this was resolved...",
            })}
          />
        </Form>
      </Modal.Content>
      <Modal.Footer>
        <Button variant={resolveAction === "resolved" ? "primary" : "danger"} onClick={onSubmit}>
          {resolveAction === "resolved" ? (
            <Trans id="action.resolve">Resolve</Trans>
          ) : (
            <Trans id="action.dismiss">Dismiss</Trans>
          )}
        </Button>
        <Button variant="tertiary" onClick={onClose}>
          <Trans id="action.cancel">Cancel</Trans>
        </Button>
      </Modal.Footer>
    </Modal.Window>
  )
}

