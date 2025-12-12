
import React from "react"
import { WebhookTriggerResult } from "@fider/models"
import { Button, Modal } from "@fider/components"

import { VStack } from "@fider/components/layout"
import { HoverInfo } from "@fider/components/common/HoverInfo"
import { WebhookProperties } from "@fider/pages/Administration/components/webhook/WebhookProperties"

interface WebhookFailInfoProps {
  result: WebhookTriggerResult
  isModalOpen: boolean
  onModalOpen: () => void
  onModalClose: () => void
}

interface InfoPropertyProps {
  value: string | number
  name: string
  info: string
  multiline?: boolean
}

const InfoProperty = (props: InfoPropertyProps) => {
  return props.value ? (
    <div className="p-3 bg-tertiary rounded-card">
      <h3 className="text-sm font-semibold text-foreground mb-2 flex items-center gap-1">
        {props.name}
        <HoverInfo text={props.info} />
      </h3>
      {props.multiline ? (
        <pre className="text-sm font-mono bg-elevated p-3 rounded-input overflow-x-auto whitespace-pre-wrap break-all m-0">{props.value}</pre>
      ) : (
        <p className="text-sm text-foreground m-0">{props.value}</p>
      )}
    </div>
  ) : (
    <div className="p-3 bg-tertiary rounded-card text-muted text-sm">
      <span className="font-medium">{props.name}</span> info not available
    </div>
  )
}

export const WebhookFailInfo = (props: WebhookFailInfoProps) => {
  return (
    <>
      <span 
        className="inline-flex items-center gap-1 px-2 py-0.5 bg-danger-light text-danger text-xs rounded-full cursor-pointer hover:bg-danger-medium transition-colors"
        onClick={props.onModalOpen}
      >
        Failed - View details
      </span>
      <Modal.Window isOpen={props.isModalOpen} onClose={props.onModalClose} size="large">
        <Modal.Header>Webhook Failure Details</Modal.Header>
        <Modal.Content>
          <VStack spacing={3}>
            <InfoProperty value={props.result.message} name="Message" info="Generic information about where it failed" />
            <InfoProperty value={props.result.error} name="Error" info="Detailed information about what failed" multiline />
            <InfoProperty value={props.result.url} name="URL" info="Parsed URL where the request has been made" />
            <InfoProperty value={props.result.content} name="Content" info="Parsed content that was sent as request body" multiline />
            <InfoProperty value={props.result.status_code} name="Status Code" info="HTTP response status code of the request" />
            <div className="p-3 bg-tertiary rounded-card">
              <h3 className="text-sm font-semibold text-foreground mb-2 flex items-center gap-1">
                Properties
                <HoverInfo text="Properties used when parsing URL and content" />
              </h3>
              <WebhookProperties properties={props.result.props} propsName="Property name" valueName="Resolved value" />
            </div>
          </VStack>
        </Modal.Content>
        <Modal.Footer>
          <Button variant="tertiary" onClick={props.onModalClose}>
            Close
          </Button>
        </Modal.Footer>
      </Modal.Window>
    </>
  )
}
