import React from "react"
import { Button } from "@fider/components"

export const controlClass = "mt-1 block w-full cursor-pointer rounded-input border border-border bg-elevated px-3 py-2 text-sm"

export function SponsorSaveButton({ busy, uncertain, disabled, onClick }: {
  busy: boolean
  uncertain: boolean
  disabled?: boolean
  onClick: () => void
}) {
  return (
    <Button variant="primary" loading={busy} disabled={disabled} onClick={onClick}>
      {uncertain ? "Retry save" : "Save"}
    </Button>
  )
}
