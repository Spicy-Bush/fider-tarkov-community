import React, { useId, useState } from "react"
import { AccountDraft, DraftPayload } from "@fider/services/browserDrafts"
import { Button } from "./Button"

interface DraftPickerProps<T extends DraftPayload> {
  drafts: AccountDraft<T>[]
  label: string
  action: string
  disabled?: boolean
  loading?: boolean
  onSelect: (draft: AccountDraft<T>) => Promise<void>
}

export function DraftPicker<T extends DraftPayload>(props: DraftPickerProps<T>) {
  const id = useId()
  const [selectedId, setSelectedId] = useState<string>()
  const selected = props.drafts.find(draft => draft.id === selectedId) || props.drafts[0]
  if (!selected) return null

  return (
    <div className="flex min-w-0 max-w-full flex-wrap items-center gap-2">
      <label htmlFor={id} className="sr-only">{props.label}</label>
      <select
        id={id}
        value={selected.id}
        disabled={props.disabled || props.loading}
        onChange={event => setSelectedId(event.target.value)}
        className="min-w-0 max-w-full cursor-pointer rounded-input border border-border bg-elevated p-2 text-sm text-body sm:max-w-sm disabled:cursor-not-allowed disabled:opacity-45"
      >
        {props.drafts.map(draft => {
          const payload = draft.payload
          const text = "title" in payload ? payload.title : "content" in payload ? payload.content : ""
          const excerpt = typeof text === "string" ? text.replace(/\s+/g, " ").trim().slice(0, 80) : ""
          const date = new Date(draft.updatedAt).toLocaleString()

          return <option key={draft.id} value={draft.id}>{excerpt ? `${excerpt} (${date})` : date}</option>
        })}
      </select>
      <Button size="small" disabled={props.disabled} loading={props.loading} onClick={() => props.onSelect(selected)}>
        {props.action}
      </Button>
    </div>
  )
}
