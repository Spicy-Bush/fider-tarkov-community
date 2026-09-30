import React, { useState } from "react"
import { Button } from "@fider/components"
import { SponsorPlacement } from "@fider/models/sponsorBooking"
import { saveSponsorPlacement } from "@fider/services/actions/sponsorBooking"
import { controlClass, SponsorSaveButton } from "./SponsorControls"
import { useSponsorSave } from "./useSponsorSave"

function PlacementEditor({ initial, onSaved }: { initial: SponsorPlacement; onSaved: (placement: SponsorPlacement) => void }) {
  const [draft, setDraft] = useState(initial)
  const save = useSponsorSave(saveSponsorPlacement, placement => {
    setDraft(placement)
    onSaved(placement)
  })
  const movable = draft.pageType === "post" || draft.pageType === "page"

  return (
    <article className="rounded-panel border border-border bg-elevated p-4">
      <h2 className="mb-4 text-base font-semibold">{draft.name}</h2>
      <fieldset disabled={save.disabled} className="space-y-4">
        <label className="flex cursor-pointer items-center gap-2 text-sm">
          <input type="checkbox" className="cursor-pointer accent-primary" checked={draft.enabled} onChange={event => setDraft({ ...draft, enabled: event.target.checked })} />
          Enabled
        </label>
        {movable && <label className="block text-sm font-medium">Position
          <select className={controlClass} value={draft.position} onChange={event => setDraft({ ...draft, position: event.target.value, every: draft.every || 4 })}>
            <option value="before">{draft.pageType === "post" ? "Below votes, above discussion" : "Before the discussion"}</option>
            <option value="within">Between discussion threads</option>
            <option value="after">After the discussion</option>
          </select>
        </label>}
        {(draft.position === "feed" || draft.position === "within") && <label className="block text-sm font-medium">
          {draft.position === "feed" ? "Posts between cards" : "Show after this many threads"}
          <input className={controlClass} type="number" min={draft.position === "feed" ? 8 : 1} max={100} value={draft.every}
            onChange={event => setDraft({ ...draft, every: Number(event.target.value) })} />
        </label>}
        <label className="block text-sm font-medium">Unbooked inventory
          <select className={controlClass} value={draft.empty} onChange={event => setDraft({ ...draft, empty: event.target.value as SponsorPlacement["empty"] })}>
            <option value="none">Leave empty</option>
            <option value="kofi">Show Ko-fi</option>
            <option value="adsense">Show Google AdSense</option>
          </select>
        </label>
        {draft.empty === "adsense" && <label className="block text-sm font-medium">AdSense ad unit ID
          <input className={controlClass} inputMode="numeric" value={draft.adsenseSlotId || ""}
            onChange={event => setDraft({ ...draft, adsenseSlotId: event.target.value })} />
        </label>}
      </fieldset>
      <div className="mt-5 flex justify-end">
        <SponsorSaveButton busy={save.busy} uncertain={save.uncertain} onClick={() => {
          if (save.uncertain) void save.retry()
          else void save.submit(draft)
        }} />
      </div>
    </article>
  )
}

export function Placements({ placements, onSaved }: { placements: SponsorPlacement[]; onSaved: (placement: SponsorPlacement) => void }) {
  const [device, setDevice] = useState("desktop")
  return (
    <div>
      <div className="mb-5 flex gap-2">
        <Button variant={device === "desktop" ? "secondary" : "tertiary"} onClick={() => setDevice("desktop")}>Desktop</Button>
        <Button variant={device === "mobile" ? "secondary" : "tertiary"} onClick={() => setDevice("mobile")}>Mobile</Button>
      </div>
      <div className="grid gap-5 xl:grid-cols-2">
        {placements.map(placement => (
          <div key={placement.id} hidden={placement.device !== device}>
            <PlacementEditor initial={placement} onSaved={onSaved} />
          </div>
        ))}
      </div>
    </div>
  )
}
