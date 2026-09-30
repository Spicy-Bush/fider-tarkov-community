import React, { useState } from "react"
import { Button, Input, TextArea } from "@fider/components"
import { SponsorshipPackage } from "@fider/models"
import { SponsorPlacement } from "@fider/models/sponsorBooking"
import { createSponsorshipPackage, updateSponsorshipPackage, deleteSponsorshipPackage } from "@fider/services/actions/sponsorship"
import { SponsorSaveButton } from "./SponsorControls"
import { useSponsorSave } from "./useSponsorSave"

const emptyPackage: SponsorshipPackage = {
  id: 0,
  name: "",
  slug: "",
  description: "",
  slots: "",
  durationDays: 28,
  sort: 0,
}

export function Packages({ initial, placements }: { initial: SponsorshipPackage[]; placements: SponsorPlacement[] }) {
  const [packages, setPackages] = useState(initial)
  const [draft, setDraft] = useState<SponsorshipPackage>()
  const save = useSponsorSave(
    (value: SponsorshipPackage, submissionId: string) => value.id
      ? updateSponsorshipPackage(value.id, { ...value, submissionId })
      : createSponsorshipPackage({ ...value, submissionId }),
    value => {
      setPackages(previous => [...previous.filter(item => item.id !== value.id), value])
      setDraft(undefined)
    },
  )
  const remove = useSponsorSave(async (id: number) => {
    const result = await deleteSponsorshipPackage(id)
    return result.ok ? { ok: true as const, data: id } : result
  }, id => setPackages(previous => previous.filter(item => item.id !== id)))

  const change = <K extends keyof SponsorshipPackage>(key: K, value: SponsorshipPackage[K]) => {
    setDraft(previous => ({ ...previous!, [key]: value }))
  }

  return (
    <section className="space-y-5">
      {draft ? (
        <div className="rounded-panel border border-border p-4">
          <fieldset disabled={save.disabled} className="space-y-4">
            <Input field="name" label="Package name" value={draft.name} onChange={value => change("name", value)} />
            <Input field="slug" label="Slug" value={draft.slug} onChange={value => change("slug", value)} />
            <TextArea field="description" label="Description" value={draft.description} onChange={value => change("description", value)} />
            <div className="grid gap-3 sm:grid-cols-2">
              {placements.map(placement => (
                <label key={placement.id} className="flex cursor-pointer items-center gap-2 text-sm">
                  <input type="checkbox" className="cursor-pointer accent-primary" checked={draft.slots.split(",").includes(placement.id)}
                    onChange={event => {
                      const selected = draft.slots.split(",").filter(Boolean)
                      change("slots", (event.target.checked ? [...selected, placement.id] : selected.filter(id => id !== placement.id)).join(","))
                    }} />
                  {placement.name}
                </label>
              ))}
            </div>
            <div className="grid gap-5 sm:grid-cols-2">
              <Input field="durationDays" label="Duration (days)" type="number" value={String(draft.durationDays)} onChange={value => change("durationDays", Number(value))} />
              <Input field="sort" label="Display order" type="number" value={String(draft.sort)} onChange={value => change("sort", Number(value))} />
            </div>
          </fieldset>
          <div className="mt-5 flex justify-end gap-2">
            <Button disabled={save.disabled} onClick={() => setDraft(undefined)}>Cancel</Button>
            <SponsorSaveButton busy={save.busy} uncertain={save.uncertain} onClick={() => {
              if (save.uncertain) void save.retry()
              else void save.submit(draft)
            }} />
          </div>
        </div>
      ) : <Button disabled={remove.disabled} onClick={() => setDraft({ ...emptyPackage })}>New package</Button>}
      {packages.map(item => (
        <article key={item.id} className="flex flex-wrap items-center justify-between gap-3 rounded-panel border border-border p-4">
          <div>
            <h2 className="m-0 text-base font-semibold">{item.name}</h2>
            <p className="mb-0 mt-1 text-sm text-muted">{item.description}</p>
          </div>
          <div className="flex gap-2">
            <Button disabled={save.disabled || remove.disabled} onClick={() => setDraft(item)}>Edit</Button>
            <Button disabled={save.disabled || remove.busy || (remove.uncertain && remove.pending !== item.id)} onClick={() => {
              if (remove.uncertain) void remove.retry()
              else if (window.confirm(`Delete ${item.name}?`)) void remove.submit(item.id)
            }}>{remove.uncertain && remove.pending === item.id ? "Retry delete" : "Delete"}</Button>
          </div>
        </article>
      ))}
    </section>
  )
}
