import React, { useState } from "react"
import { Button } from "@fider/components"
import { Webhook, WebhookData, WebhookStatus } from "@fider/models"
import { actions, Failure } from "@fider/services"
import { WebhookForm } from "../components/webhook/WebhookForm"
import { WebhookListItem } from "../components/webhook/WebhookListItem"
import { WebhookDocsPanel } from "../components/webhook/WebhookDocsPanel"
import { VStack, HStack } from "@fider/components/layout"
import { PageConfig } from "@fider/components/layouts"

export const pageConfig: PageConfig = {
  title: "Webhooks",
  subtitle: "Manage your site webhooks",
  sidebarItem: "webhooks",
}

interface ManageWebhooksPageProps {
  webhooks: Webhook[]
}

const webhookSorter = (w1: Webhook, w2: Webhook) => {
  if (w1.name < w2.name) {
    return -1
  } else if (w1.name > w2.name) {
    return 1
  }
  return 0
}

interface WebhooksListProps {
  title: string
  description: string
  list: JSX.Element[]
}

const WebhooksList = (props: WebhooksListProps) => {
  return (
    <div>
      <h2 className="text-lg font-semibold text-foreground mb-4">My Webhooks</h2>
      <VStack spacing={3}>
        {props.list.length === 0 ? (
          <div className="p-8 text-center bg-tertiary rounded-card border border-surface-alt">
            <p className="text-muted m-0">There aren&apos;t any webhooks yet.</p>
          </div>
        ) : props.list}
      </VStack>
    </div>
  )
}

const ManageWebhooksPage: React.FC<ManageWebhooksPageProps> = (props) => {
  const [isAdding, setIsAdding] = useState(false)
  const [allWebhooks, setAllWebhooks] = useState(() => [...props.webhooks].sort(webhookSorter))
  const [editing, setEditing] = useState<Webhook | undefined>()
  const [isDocsOpen, setIsDocsOpen] = useState(false)

  const sortWebhooks = () => setAllWebhooks(prev => [...prev].sort(webhookSorter))

  const addNew = () => {
    setIsAdding(true)
    setEditing(undefined)
  }
  const cancelAdd = () => setIsAdding(false)

  const saveNewWebhook = async (data: WebhookData): Promise<Failure | undefined> => {
    const result = await actions.createWebhook(data)
    if (result.ok) {
      setIsAdding(false)
      setAllWebhooks(prev => [...prev, { id: result.data.id, ...data }].sort(webhookSorter))
    } else {
      return result.error
    }
  }

  const startWebhookEditing = (webhook: Webhook) => {
    setIsAdding(false)
    setEditing(webhook)
  }
  const cancelEdit = () => setEditing(undefined)

  const handleWebhookDeleted = (webhook: Webhook) => {
    setAllWebhooks(prev => prev.filter(w => w.id !== webhook.id))
  }

  const handleWebhookEdited = async (data: WebhookData): Promise<Failure | undefined> => {
    const webhook = editing
    if (webhook === undefined) return
    const result = await actions.updateWebhook(webhook.id, data)
    if (result.ok) {
      webhook.name = data.name
      webhook.type = data.type
      webhook.status = data.status === WebhookStatus.FAILED ? WebhookStatus.DISABLED : data.status
      webhook.url = data.url
      webhook.content = data.content
      webhook.http_method = data.http_method
      webhook.http_headers = data.http_headers

      setEditing(undefined)
      sortWebhooks()
    } else {
      return result.error
    }
  }

  const handleWebhookFailed = (webhook: Webhook) => {
    webhook.status = WebhookStatus.FAILED
    sortWebhooks()
  }

  const handleWebhookStatusChanged = (webhook: Webhook) => {
    sortWebhooks()
  }

  const getWebhookItems = () => {
    return allWebhooks.map((w) => (
      <WebhookListItem
        key={w.id}
        webhook={w}
        onWebhookDeleted={handleWebhookDeleted}
        editWebhook={startWebhookEditing}
        onWebhookFailed={handleWebhookFailed}
        onWebhookStatusChanged={handleWebhookStatusChanged}
      />
    ))
  }

  if (isAdding) {
    return <WebhookForm onSave={saveNewWebhook} onCancel={cancelAdd} />
  }

  if (editing) {
    return <WebhookForm onSave={handleWebhookEdited} onCancel={cancelEdit} webhook={editing} />
  }

  return (
    <>
      <VStack spacing={6}>
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3 p-4 bg-tertiary rounded-card border border-surface-alt">
          <p className="text-muted m-0">
            Use webhooks to integrate Fider with other applications like Slack, Discord, Zapier and many others.{" "}
            <a className="text-link" href="https://fider.io/docs/using-webhooks" target="_blank" rel="noopener">
              Learn more
            </a>
          </p>
          <Button variant="secondary" size="small" onClick={() => setIsDocsOpen(true)} className="shrink-0">
            Docs
          </Button>
        </div>
        <WebhooksList title="New Post" description="a new post is created on this site" list={getWebhookItems()} />
        <div className="pt-4 border-t border-surface-alt">
          <Button variant="primary" onClick={addNew}>
            Add new webhook
          </Button>
        </div>
      </VStack>
      <WebhookDocsPanel isOpen={isDocsOpen} onClose={() => setIsDocsOpen(false)} />
    </>
  )
}

export default ManageWebhooksPage
