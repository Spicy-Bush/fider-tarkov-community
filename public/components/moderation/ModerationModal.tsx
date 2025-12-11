import React, { useState, useEffect } from "react"
import { Modal, Form, TextArea, Input, Button, Loader } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { Failure, actions } from "@fider/services"
import { CannedResponse } from "@fider/services/actions/response"

interface ModerationModalProps {
  isOpen: boolean
  onClose: () => void
  actionType: 'mute' | 'warning'
  onSubmit: (data: { reason: string; duration: string }) => Promise<void>
  error?: Failure
}

const DURATION_PRESETS = [
  { label: "30m", value: "30m" },
  { label: "1h", value: "1h" },
  { label: "1d", value: "1d" },
  { label: "7d", value: "7d" },
  { label: "30d", value: "30d" },
  { label: "1y", value: "1y" },
  { label: "Soft Ban (10y)", value: "10y" },
]

export const ModerationModal: React.FC<ModerationModalProps> = ({ isOpen, onClose, actionType, onSubmit, error }) => {
  const [reason, setReason] = useState("")
  const [duration, setDuration] = useState(actionType === 'warning' ? "0" : "1h")
  const [isCustomReason, setIsCustomReason] = useState(true)
  const [cannedResponses, setCannedResponses] = useState<CannedResponse[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [loadError, setLoadError] = useState<string>()

  useEffect(() => {
    if (isOpen) {
      setReason("")
      setDuration(actionType === 'warning' ? "0" : "1h")
      setIsCustomReason(true)
      
      setIsLoading(true)
      setLoadError(undefined)
      
      actions.listCannedResponses(actionType)
        .then(result => {
          if (result.ok) {
            setCannedResponses(result.data || [])
          } else {
            setLoadError("Failed to load canned responses")
            setCannedResponses([])
          }
          setIsLoading(false)
        })
        .catch(() => {
          setLoadError("Failed to load canned responses")
          setCannedResponses([])
          setIsLoading(false)
        })
    }
  }, [isOpen, actionType])

  const handleDurationSelect = (value: string) => {
    setDuration(value)
  }

  const handleCannedResponseSelect = (response: CannedResponse) => {
    setReason(response.content)
    if (response.duration) {
      setDuration(response.duration)
    }
    setIsCustomReason(false)
  }

  const handleSubmit = async () => {
    if (reason.trim()) {
      await onSubmit({ reason, duration })
    }
  }

  return (
    <Modal.Window isOpen={isOpen} onClose={onClose} center={false} size="large">
      <Modal.Header>
        {actionType === 'warning' ? (
          <Trans id="moderation.warning.header">Warn User</Trans>
        ) : (
          <Trans id="moderation.mute.header">Mute User</Trans>
        )}
      </Modal.Header>
      
      <Modal.Content>
        <Form error={error}>
          <div className="mb-4">
            <label className="text-medium">
              <Trans id="moderation.canned.label">Canned Responses</Trans>
            </label>
            {isLoading ? (
              <div className="py-4 text-center">
                <Loader />
              </div>
            ) : loadError ? (
              <div className="py-2 text-red-500">{loadError}</div>
            ) : (
              <div className="grid grid-cols-2 gap-2 mt-2">
                {cannedResponses && cannedResponses.length > 0 ? (
                  cannedResponses.map((response) => (
                    <Button 
                      key={response.id} 
                      variant="secondary" 
                      className={!isCustomReason && reason === response.content ? "" : ""}
                      onClick={() => handleCannedResponseSelect(response)}
                    >
                      {response.title}
                    </Button>
                  ))
                ) : (
                  <div className="col-span-2 py-2 text-muted">
                    <Trans id="moderation.canned.empty">No canned responses available.</Trans>
                  </div>
                )}
                <Button 
                  variant="secondary" 
                  className={isCustomReason ? "" : ""}
                  onClick={() => setIsCustomReason(true)}
                >
                  <Trans id="moderation.custom.response">Custom</Trans>
                </Button>
              </div>
            )}
          </div>

          <TextArea
            field="reason"
            label={actionType === 'warning' ? (
              i18n._("moderation.warning.reason.label", { message: "Warning Reason" })
            ) : (
              i18n._("moderation.mute.reason.label", { message: "Mute Reason" })
            )}
            value={reason}
            onChange={setReason}
            placeholder={actionType === 'warning' ? (
              i18n._("moderation.warning.reason.placeholder", { message: "Enter reason for warning user..." })
            ) : (
              i18n._("moderation.mute.reason.placeholder", { message: "Enter reason for muting user..." })
            )}
          />
          
          <div className="mt-4">
            <label className="text-medium mb-2 block">
              {actionType === 'warning' ? (
                <Trans id="moderation.warning.duration.label">Warning Duration</Trans>
              ) : (
                <Trans id="moderation.mute.duration.label">Mute Duration</Trans>
              )}
            </label>
            
            <HStack spacing={2} className="mb-2 flex-wrap">
              {DURATION_PRESETS.map((preset) => (
                <Button 
                  key={preset.value} 
                  variant="secondary" 
                  size="small"
                  className={duration === preset.value ? "" : ""}
                  onClick={() => handleDurationSelect(preset.value)}
                >
                  {preset.label}
                </Button>
              ))}
            </HStack>
            
            <Input
              field="duration"
              placeholder={actionType === 'warning' ? (
                i18n._("moderation.warning.duration.placeholder", { message: "0 for permanent, or 30m, 1h, 7d, etc." })
              ) : (
                i18n._("moderation.mute.duration.placeholder", { message: "e.g. 30m, 1h, 7d, etc." })
              )}
              value={duration}
              onChange={setDuration}
            />
            <p className="text-muted text-xs mt-1">
              <Trans id="moderation.duration.help">
                Formats: m (minutes), h (hours), d (days), w (weeks), M (months), y (years)
              </Trans>
            </p>
          </div>
        </Form>
      </Modal.Content>

      <Modal.Footer>
        <Button variant="primary" disabled={!reason.trim()} onClick={handleSubmit}>
          {actionType === 'warning' ? (
            <Trans id="action.warning">Warn User</Trans>
          ) : (
            <Trans id="action.mute">Mute User</Trans>
          )}
        </Button>
        <Button variant="tertiary" onClick={onClose}>
          <Trans id="action.cancel">Cancel</Trans>
        </Button>
      </Modal.Footer>
    </Modal.Window>
  )
}

