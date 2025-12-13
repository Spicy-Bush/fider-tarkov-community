
import React, { useState } from "react"
import { Webhook, WebhookStatus, WebhookTriggerResult, WebhookType } from "@fider/models"
import { Button, Icon, Toggle } from "@fider/components"
import { actions, notify, classSet } from "@fider/services"

import { heroiconsX as IconX, heroiconsPencilAlt as IconPencilAlt, heroiconsPlay as IconPlay, heroiconsCheckCircle as IconCheckCircle, heroiconsXCircle as IconXCircle, heroiconsExclamation as IconExclamation } from "@fider/icons.generated"
import { HStack } from "@fider/components/layout"
import { WebhookFailInfo } from "./WebhookFailInfo"

interface WebhookListItemProps {
  webhook: Webhook
  editWebhook: (webhook: Webhook) => void
  onWebhookDeleted: (webhook: Webhook) => void
  onWebhookFailed: (webhook: Webhook) => void
  onWebhookStatusChanged: (webhook: Webhook) => void
}

const getStatusIcon = (status: WebhookStatus) => {
  switch (status) {
    case WebhookStatus.ENABLED:
      return { icon: IconCheckCircle, colorClass: "text-success", tooltip: "Enabled" }
    case WebhookStatus.DISABLED:
      return { icon: IconXCircle, colorClass: "text-muted", tooltip: "Disabled" }
    case WebhookStatus.FAILED:
      return { icon: IconExclamation, colorClass: "text-danger", tooltip: "Failed" }
    default:
      return { icon: IconXCircle, colorClass: "text-muted", tooltip: "Unknown" }
  }
}

export const WebhookListItem = (props: WebhookListItemProps) => {
  const [deleting, setDeleting] = useState(false)
  const [triggerResult, setTriggerResult] = useState<WebhookTriggerResult | undefined>(undefined)
  const [isFailInfoModalOpen, setIsFailInfoModalOpen] = useState(false)

  const showFailInfoModal = () => setIsFailInfoModalOpen(true)
  const hideFailInfoModal = () => setIsFailInfoModalOpen(false)

  const deleteWebhook = async () => {
    const result = await actions.deleteWebhook(props.webhook.id)
    if (result.ok) {
      setDeleting(false)
      props.onWebhookDeleted(props.webhook)
    }
  }

  const getWebhookType = (type: WebhookType) => {
    switch (type) {
      case WebhookType.CHANGE_STATUS:
        return "Change Status"
      case WebhookType.NEW_COMMENT:
        return "New Comment"
      case WebhookType.DELETE_POST:
        return "Delete Post"
      case WebhookType.NEW_POST:
        return "New Post"
      case WebhookType.NEW_REPORT:
        return "New Report"
      case WebhookType.REPORT_RESOLVED:
        return "Report Resolved"
    }
  }

  const testWebhook = async () => {
    const result = await actions.testWebhook(props.webhook.id)
    setTriggerResult(result.data)
    if (result.ok && result.data.success) {
      notify.success("Successfully triggered webhook")
    } else {
      notify.error(result.data.message)
      props.onWebhookFailed(props.webhook)
    }
  }

  const toggleWebhookStatus = async (active: boolean) => {
    const newStatus = active ? WebhookStatus.ENABLED : WebhookStatus.DISABLED
    const result = await actions.updateWebhook(props.webhook.id, {
      ...props.webhook,
      status: newStatus,
    })
    if (result.ok) {
      props.webhook.status = newStatus
      props.onWebhookStatusChanged(props.webhook)
      notify.success(`Webhook ${active ? "enabled" : "disabled"}`)
    } else {
      notify.error("Failed to update webhook status")
    }
  }

  const renderDeleteMode = () => {
    return (
      <div className="p-4 bg-danger-light border border-danger-light rounded-card">
        <div className="mb-3">
          <span className="font-semibold text-danger">Are you sure?</span>{" "}
          <span className="text-foreground">
            The webhook #{props.webhook.id} &quot;{props.webhook.name}&quot; will be deleted forever. Alternatively, you may want to <span className="font-medium">disable</span> it instead.
          </span>
        </div>
        <HStack spacing={2}>
          <Button variant="danger" size="small" onClick={deleteWebhook}>
            Delete
          </Button>
          <Button onClick={() => setDeleting(false)} variant="tertiary" size="small">
            Cancel
          </Button>
        </HStack>
      </div>
    )
  }

  const renderViewMode = () => {
    const statusIcon = getStatusIcon(props.webhook.status)
    
    return (
      <div className="flex items-center justify-between gap-4 py-4 px-5 bg-elevated border border-surface-alt rounded-card hover:border-border-strong transition-colors">
        <div className="flex items-center gap-4 min-w-0">
          <div className="shrink-0 w-7 h-7 flex items-center justify-center" data-tooltip={statusIcon.tooltip}>
            <Icon sprite={statusIcon.icon} className={`w-7 h-7 ${statusIcon.colorClass}`} />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2 mb-0.5">
              <span className="text-sm text-muted">#{props.webhook.id}</span>
              <span className={classSet({
                "text-xs px-2 py-0.5 rounded font-medium": true,
                "bg-info-light text-primary": props.webhook.type === WebhookType.NEW_POST,
                "bg-success-light text-success": props.webhook.type === WebhookType.NEW_COMMENT,
                "bg-warning-light text-warning": props.webhook.type === WebhookType.CHANGE_STATUS,
                "bg-danger-light text-danger": props.webhook.type === WebhookType.DELETE_POST,
                "bg-surface-alt text-muted": props.webhook.type === WebhookType.NEW_REPORT || props.webhook.type === WebhookType.REPORT_RESOLVED,
              })}>
                {getWebhookType(props.webhook.type)}
              </span>
              {triggerResult?.success === false && (
                <WebhookFailInfo result={triggerResult} isModalOpen={isFailInfoModalOpen} onModalOpen={showFailInfoModal} onModalClose={hideFailInfoModal} />
              )}
            </div>
            <h3 className="text-foreground font-medium truncate">{props.webhook.name}</h3>
          </div>
        </div>
        <div className="flex items-center gap-3 shrink-0">
          <Toggle
            active={props.webhook.status === WebhookStatus.ENABLED}
            onToggle={toggleWebhookStatus}
            disabled={props.webhook.status === WebhookStatus.FAILED}
          />
          <Button size="small" variant="secondary" onClick={testWebhook}>
            <Icon sprite={IconPlay} className="w-4 h-4" />
            <span>Test</span>
          </Button>
          <Button size="small" variant="secondary" onClick={() => props.editWebhook(props.webhook)}>
            <Icon sprite={IconPencilAlt} className="w-4 h-4" />
            <span>Edit</span>
          </Button>
          <button 
            type="button"
            onClick={() => setDeleting(true)}
            className="p-2 rounded-badge text-muted hover:text-danger hover:bg-danger-light transition-colors cursor-pointer border-none bg-transparent"
          >
            <Icon sprite={IconX} className="w-4 h-4" />
          </button>
        </div>
      </div>
    )
  }

  return deleting ? renderDeleteMode() : renderViewMode()
}
