
import React, { useEffect, useState } from "react"
import { Button, Field, Form, Input, Loader, Message, Select, SelectOption, TextArea, Toggle } from "@fider/components"
import { actions, Failure } from "@fider/services"
import { HStack, VStack } from "@fider/components/layout"
import { Webhook, WebhookData, WebhookPreviewResult, WebhookStatus, WebhookType } from "@fider/models"
import { HoverInfo } from "@fider/components/common/HoverInfo"
import { WebhookTemplateInfoModal } from "@fider/pages/Administration/components/webhook/WebhookTemplateInfoModal"
import { WebhookDocsPanel } from "@fider/pages/Administration/components/webhook/WebhookDocsPanel"

interface WebhookFormProps {
  webhook?: Webhook
  onSave: (data: WebhookData) => Promise<Failure | undefined>
  onCancel: () => void
}

interface HttpHeaderProps {
  header?: string
  value?: string
  onEdit: (header: string, value: string) => void
  onRemove?: (header: string) => void
  allHeaders?: string[]
}

const HttpHeader = (props: HttpHeaderProps) => {
  const [header, setHeader] = useState(props.header || "")
  const [value, setValue] = useState(props.value || "")
  const [editing, setEditing] = useState(props.onRemove === undefined)

  const duplicate = props.allHeaders && props.allHeaders.includes(header)

  const suffix = editing ? (
    props.onRemove === undefined ? (
      <Button
        variant="primary"
        onClick={() => {
          props.onEdit(header, value)
          setHeader("")
          setValue("")
        }}
        disabled={duplicate || header.length === 0 || value.length === 0}
      >
        Add
      </Button>
    ) : (
      <Button
        variant="primary"
        onClick={() => {
          setEditing(false)
          props.onEdit(header, value)
        }}
        disabled={value.length === 0}
      >
        Save
      </Button>
    )
  ) : (
    <>
      <Button variant="secondary" onClick={() => setEditing(true)}>
        Edit
      </Button>
      <Button variant="danger" onClick={() => props.onRemove && props.onRemove(header)}>
        Remove
      </Button>
    </>
  )

  return (
    <HStack justify="full" spacing={4}>
      <Input
        field={`header-${header}`}
        value={header}
        onChange={setHeader}
        placeholder="Header"
        disabled={props.onRemove !== undefined}
        suffix={duplicate ? "Duplicate" : undefined}
      />
      <Input field={`value-${header}`} value={value} onChange={setValue} placeholder="Value" disabled={!editing} suffix={suffix} />
    </HStack>
  )
}

export const WebhookForm = (props: WebhookFormProps) => {
  const [name, setName] = useState(props.webhook?.name || "")
  const [type, _setType] = useState(props.webhook?.type || WebhookType.NEW_POST)
  const [status, _setStatus] = useState(props.webhook?.status || WebhookStatus.DISABLED)
  const [url, setUrl] = useState(props.webhook?.url || "")
  const [content, setContent] = useState(props.webhook?.content || "")
  const [httpMethod, setHttpMethod] = useState(props.webhook?.http_method || "POST")
  const [httpHeaders, _setHttpHeaders] = useState(props.webhook?.http_headers || {})
  const [typing, setTyping] = useState<NodeJS.Timeout | undefined>()
  const [preview, setPreview] = useState<WebhookPreviewResult | null | undefined>()
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [isDocsOpen, setIsDocsOpen] = useState(false)
  const [error, setError] = useState<Failure | undefined>()

  const calculatePreview = () => {
    actions
      .previewWebhook(type, url, content)
      .then(
        (result) => (result.ok ? result.data : null),
        () => null
      )
      .then(setPreview)
  }

  useEffect(calculatePreview, [])
  useEffect(() => {
    if (typing) clearTimeout(typing)
    setPreview(undefined)
    setTyping(
      setTimeout(() => {
        calculatePreview()
        setTyping(undefined)
      }, 2_000)
    )
  }, [url, content])

  const handleSave = async () => {
    const error = await props.onSave({ name, type, status, url, content, http_method: httpMethod, http_headers: httpHeaders })
    if (error) {
      setError(error)
    }
  }
  const handleCancel = () => props.onCancel()

  const setType = (option?: SelectOption) => _setType(option?.value as WebhookType)
  const setStatus = (active: boolean) => _setStatus(active ? WebhookStatus.ENABLED : WebhookStatus.DISABLED)

  const setHttpHeader = (header: string, value: string) => {
    _setHttpHeaders((headers) => ({
      ...headers,
      [header]: value,
    }))
  }

  const removeHttpHeader = (header: string) => {
    _setHttpHeaders((headers) => {
      const { [header]: _, ...remaining } = headers
      return remaining
    })
  }

  const showModal = () => setIsModalOpen(true)
  const hideModal = () => setIsModalOpen(false)
  const showDocs = () => setIsDocsOpen(true)
  const hideDocs = () => setIsDocsOpen(false)

  const allHeaders = Object.keys(httpHeaders)
  const title = props.webhook ? `Webhook #${props.webhook.id}: ${props.webhook.name}` : "New webhook"
  return (
    <>
      {status === WebhookStatus.FAILED && (
        <Message type="error" showIcon>
          This webhook has failed
        </Message>
      )}
      <div className="flex items-center justify-between mb-6 pb-4 border-b border-surface-alt">
        <h2 className="text-xl font-semibold text-foreground m-0">{title}</h2>
        <Button variant="secondary" size="small" onClick={showDocs}>
          Docs
        </Button>
      </div>
      <Form error={error}>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
          <Input field="name" label="Name" value={name} onChange={setName} placeholder="My awesome webhook" />
          <Select
            label="Type"
            field="type"
            defaultValue={type}
            options={[
              { label: "New Post", value: WebhookType.NEW_POST },
              { label: "New Comment", value: WebhookType.NEW_COMMENT },
              { label: "Change Status", value: WebhookType.CHANGE_STATUS },
              { label: "Delete Post", value: WebhookType.DELETE_POST },
              { label: "New Report", value: WebhookType.NEW_REPORT },
              { label: "Report Resolved", value: WebhookType.REPORT_RESOLVED },
            ]}
            onChange={setType}
          />
        </div>
        <Field label="Enabled">
          <Toggle active={status === WebhookStatus.ENABLED} onToggle={setStatus} />
          {status === WebhookStatus.FAILED && <p className="text-sm text-danger mt-1">This webhook was disabled due to a trigger failure</p>}
        </Field>
        <Input
          field="url"
          label="URL"
          afterLabel={<HoverInfo text="You can use Go template formatting with many properties here" onClick={showModal} />}
          value={url}
          onChange={setUrl}
          placeholder="https://webhook.site/..."
        />
        <TextArea
          field="content"
          label="Content"
          afterLabel={<HoverInfo text="You can use Go template formatting with many properties here" onClick={showModal} />}
          value={content}
          onChange={setContent}
          placeholder="Request body"
          minRows={6}
        />
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Input field="http_method" label="HTTP Method" value={httpMethod} onChange={setHttpMethod} placeholder="POST" />
        </div>
        <Field label="HTTP Headers" afterLabel={<HoverInfo text="Those headers are sent in the request when the webhook is triggered" />}>
          <div className="bg-tertiary rounded-card p-3 border border-surface-alt">
            <VStack spacing={2}>
              {Object.entries(httpHeaders).map(([header, value]) => (
                <HttpHeader key={header} header={header} value={value} onEdit={setHttpHeader} onRemove={removeHttpHeader} />
              ))}
              <HttpHeader onEdit={setHttpHeader} allHeaders={allHeaders} />
            </VStack>
          </div>
        </Field>
        {(url || content) && (
          <Field label="Preview">
            {preview === null ? (
              <div className="p-4 bg-danger-light border border-danger-light rounded-card text-danger">
                Failed to load preview
              </div>
            ) : preview === undefined ? (
              <div className="p-4 bg-tertiary rounded-card border border-surface-alt">
                <Loader className="text-center" text="Loading preview" />
              </div>
            ) : (
              <div className="bg-tertiary rounded-card border border-surface-alt overflow-hidden">
                {url && (
                  <div className="p-4 border-b border-surface-alt last:border-b-0">
                    <h3 className="text-sm font-semibold text-foreground mb-2">URL</h3>
                    <pre className={`text-sm font-mono p-3 rounded-input overflow-x-auto whitespace-pre-wrap break-all m-0 ${preview.url.error ? "bg-danger-light text-danger" : "bg-elevated"}`}>
                      {preview.url.value || preview.url.error}
                    </pre>
                    {preview.url.message && <p className="text-sm text-muted mt-2">{preview.url.message}</p>}
                  </div>
                )}
                {content && (
                  <div className="p-4">
                    <h3 className="text-sm font-semibold text-foreground mb-2">Content</h3>
                    <pre className={`text-sm font-mono p-3 rounded-input overflow-x-auto whitespace-pre-wrap break-all m-0 ${preview.content.error ? "bg-danger-light text-danger" : "bg-elevated"}`}>
                      {preview.content.value || preview.content.error}
                    </pre>
                    {preview.content.message && <p className="text-sm text-muted mt-2">{preview.content.message}</p>}
                  </div>
                )}
              </div>
            )}
          </Field>
        )}
        <div className="flex gap-2 pt-4 border-t border-surface-alt mt-6">
          <Button variant="primary" onClick={handleSave}>
            Save
          </Button>
          <Button onClick={handleCancel} variant="tertiary">
            Cancel
          </Button>
        </div>
      </Form>
      <WebhookTemplateInfoModal type={type} isModalOpen={isModalOpen} onModalClose={hideModal} />
      <WebhookDocsPanel isOpen={isDocsOpen} onClose={hideDocs} />
    </>
  )
}
