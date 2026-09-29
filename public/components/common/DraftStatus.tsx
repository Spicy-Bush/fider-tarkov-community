import React from "react"
import { AccountDraft, DraftPayload } from "@fider/services/browserDrafts"
import { DraftPicker } from "./DraftPicker"
import { DisplayError } from "./form/DisplayError"

interface DraftStatusProps<T extends DraftPayload> {
  status: "idle" | "loading" | "saving" | "saved" | "error"
  error?: string
  alternatives: AccountDraft<T>[]
  flush: () => Promise<T>
  resume: (draft: AccountDraft<T>) => Promise<void>
}

export function DraftStatus<T extends DraftPayload>(props: DraftStatusProps<T>) {
  if (props.status !== "error" && props.alternatives.length === 0) {
    return null
  }

  return (
    <div className="my-2 flex flex-wrap items-center gap-2 text-xs text-muted" aria-live="polite">
      {props.status === "error" && (
        <>
          <DisplayError error={{ errors: [{ message: props.error || "Could not save this draft." }] }} />
          <button
            type="button"
            className="text-link underline"
            onClick={() => {
              void props.flush().catch(() => {})
            }}
          >
            Retry draft save
          </button>
        </>
      )}
      <DraftPicker
        drafts={props.alternatives}
        label="Saved drafts"
        action="Restore draft"
        disabled={props.status === "saving"}
        onSelect={draft => props.resume(draft).catch(() => {})}
      />
    </div>
  )
}
