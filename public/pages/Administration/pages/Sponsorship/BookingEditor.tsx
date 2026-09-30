import React, { useState } from "react"
import { Button, Input, TextArea } from "@fider/components"
import { SponsorCampaign, SponsorCampaignSave, SponsorCreative, SponsorPlacement, SponsorSaveConflict, SponsorConflicts } from "@fider/models/sponsorBooking"
import { saveSponsorCampaign } from "@fider/services/actions/sponsorBooking"
import { utcToDatetimeLocalValue } from "@fider/components/sponsorship/datetime"
import { controlClass, SponsorSaveButton } from "./SponsorControls"
import { useSponsorSave } from "./useSponsorSave"
import { CreativeFields, newSponsorCreative } from "./CreativeEditor"
import { SponsorEditor, SponsorPreview } from "./SponsorPreview"
import * as notify from "@fider/services/notify"

const campaignLabels: Record<NonNullable<SponsorConflicts["campaign"]>[number], string> = {
  name: "Booking name",
  advertiser: "Advertiser",
  category: "Category",
  exclusive: "Category exclusivity",
  state: "Booking status",
  startAt: "Start",
  endAt: "End",
  confirmBy: "Confirmation deadline",
  languages: "Languages",
  countries: "Countries",
  pageTypes: "Page types",
  amountMinor: "Amount",
  currency: "Currency",
  paymentStatus: "Payment status",
  notes: "Notes",
}

const creativeLabels: Record<NonNullable<SponsorConflicts["creative"]>[number], string> = {
  state: "Review status",
  reviewReason: "Review reason",
  language: "Artwork language",
  device: "Artwork device",
  startAt: "Artwork start",
  endAt: "Artwork end",
  framed: "Image frame",
  headline: "Headline",
  description: "Description",
  logoKey: "Logo",
  callToAction: "Call to action",
  destination: "Destination",
  offerCode: "Offer code",
  offerTerms: "Offer terms",
  offerExpires: "Offer expiry",
}

function hasConflicts(fields: SponsorConflicts): boolean {
  return !!(fields.image || fields.campaign?.length || fields.creative?.length || fields.bookings?.length)
}

function savedValue(field: string, value: string | string[] | boolean | number | null): React.ReactNode {
  if (value === null || value === "") return "Not set"
  if (typeof value === "boolean") return value ? "Yes" : "No"
  if (Array.isArray(value)) return value.length ? value.join(", ") : "All"
  if (field === "amountMinor") return (Number(value) / 100).toFixed(2)

  if (field === "logoKey") {
    return <img src={`/static/images/${value}?size=200`} alt="Saved logo" className="h-16 w-24 object-contain" />
  }

  if (["startAt", "endAt", "confirmBy", "offerExpires"].includes(field)) {
    return new Date(String(value)).toLocaleString()
  }

  return String(value)
}

export function newSponsorCampaign(): SponsorCampaign {
  const start = new Date()

  return {
    id: 0,
    revision: 0,
    name: "",
    advertiser: "",
    category: "",
    exclusive: false,
    state: "draft",
    startAt: start.toISOString(),
    endAt: new Date(start.getTime() + 28 * 86400000).toISOString(),
    confirmBy: null,
    languages: [],
    countries: [],
    pageTypes: ["home", "post", "page"],
    bookings: [],
    amountMinor: 0,
    currency: "AUD",
    paymentStatus: "unpaid",
    notes: "",
  }
}

export function BookingEditor({ initial, initialCreative, initialConflict, placements, onSaved, onRefreshed, onClose }: {
  initial: SponsorCampaign
  initialCreative?: SponsorCreative
  initialConflict?: SponsorSaveConflict
  placements: SponsorPlacement[]
  onSaved: (saved: SponsorCampaignSave) => void
  onRefreshed: (saved: SponsorCampaignSave) => void
  onClose: () => void
}) {
  const [base, setBase] = useState({ campaign: initial, creative: initialCreative })
  const [conflict, setConflict] = useState(initialConflict && hasConflicts(initialConflict.fields) ? initialConflict : undefined)
  const [draft, setDraft] = useState(initialConflict?.draft.campaign ?? initial)
  const [countries, setCountries] = useState((initialConflict?.draft.campaign ?? initial).countries.join(", "))
  const [creative, setCreative] = useState(() => initialConflict?.draft.creative ?? initialCreative ?? newSponsorCreative(initial.id))
  const [includeCreative, setIncludeCreative] = useState(!!initialCreative || !initial.id)
  const [pendingImages, setPendingImages] = useState({ image: false, logo: false })
  const uploading = includeCreative && (pendingImages.image || pendingImages.logo)
  const save = useSponsorSave(saveSponsorCampaign, result => {
    if (result.kind === "saved") {
      onSaved(result.saved)
      return
    }

    setBase({ campaign: result.saved.campaign, creative: result.saved.creative ?? undefined })
    onRefreshed(result.saved)
    setDraft(result.draft.campaign)
    setCountries((result.draft.campaign.countries ?? []).join(", "))
    if (result.draft.creative) setCreative(result.draft.creative)
    setConflict(hasConflicts(result.fields) ? result : undefined)
    for (const problem of result.problems) notify.error(problem)
  })

  const clearConflict = (target: keyof SponsorConflicts, field?: string) => {
    setConflict(previous => {
      const fields = {
        ...previous!.fields,
        [target]: target === "image" ? false : previous!.fields[target]?.filter(item => item !== field),
      }

      return hasConflicts(fields) ? { ...previous!, fields } : undefined
    })
  }

  const conflictChoice = (target: keyof SponsorConflicts, field: string, label: string, value: React.ReactNode, useSaved: () => void) => (
    <div key={target + field} className="flex flex-wrap items-center justify-between gap-3">
      <div className="min-w-0 text-sm"><strong>{label}</strong><div className="break-words text-muted">{value}</div></div>
      <div className="flex gap-2">
        <Button onClick={() => clearConflict(target, field)}>Keep mine</Button>
        <Button onClick={() => { useSaved(); clearConflict(target, field) }}>Use saved</Button>
      </div>
    </div>
  )

  const change = <K extends keyof SponsorCampaign>(key: K, value: SponsorCampaign[K]) => {
    setDraft(previous => ({ ...previous, [key]: value }))
  }

  const changeDate = (key: "startAt" | "endAt" | "confirmBy", value: string) => {
    if (!value) {
      change(key, key === "confirmBy" ? null : "")
      return
    }

    const date = new Date(value)
    if (Number.isFinite(date.getTime())) {
      change(key, date.toISOString())
    }
  }

  return (
    <SponsorEditor
      title={initial.id ? "Edit campaign" : "New campaign"}
      disabled={save.disabled || uploading}
      onClose={onClose}
      preview={includeCreative ? <SponsorPreview creative={creative} advertiser={draft.advertiser} placements={placements} bookings={draft.bookings} /> : undefined}
    >
      {conflict && (
        <section aria-label="Conflicting changes" className="mb-6 space-y-3 rounded-card border border-border p-4">
          <h3 className="m-0 font-semibold">Choose which changes to keep</h3>
          {conflict.fields.campaign?.map(field => conflictChoice(
            "campaign", field, campaignLabels[field], savedValue(field, conflict.saved.campaign[field]), () => {
              setDraft(previous => ({ ...previous, [field]: conflict.saved.campaign[field] }))
              if (field === "countries") setCountries(conflict.saved.campaign.countries.join(", "))
            },
          ))}
          {conflict.fields.creative?.map(field => conflictChoice(
            "creative", field, creativeLabels[field], savedValue(field, conflict.saved.creative![field]), () =>
              setCreative(previous => ({ ...previous, [field]: conflict.saved.creative![field] })),
          ))}
          {conflict.fields.bookings?.map(id => {
            const booking = conflict.saved.campaign.bookings.find(item => item.placementId === id)
            return conflictChoice("bookings", id, placements.find(item => item.id === id)!.name,
              booking ? `${booking.share}%` : "Not booked", () => setDraft(previous => ({
                ...previous,
                bookings: [...previous.bookings.filter(item => item.placementId !== id), ...(booking ? [booking] : [])],
              })),
            )
          })}
          {conflict.fields.image && conflictChoice("image", "", "Artwork image",
            conflict.saved.creative!.imageKey
              ? <img src={`/static/images/${conflict.saved.creative!.imageKey}?size=200`} alt="Saved image" className="mt-2 h-20 w-32 object-contain" />
              : "No image",
            () => setCreative(previous => ({
              ...previous,
              imageKey: conflict.saved.creative!.imageKey,
              bannerCrop: conflict.saved.creative!.bannerCrop,
            })),
          )}
        </section>
      )}
      <fieldset disabled={save.disabled} className="grid min-w-0 gap-x-5 sm:grid-cols-2">
        <Input field="name" label="Booking name" value={draft.name} onChange={value => change("name", value)} />
        <Input field="advertiser" label="Advertiser" value={draft.advertiser} onChange={value => change("advertiser", value)} />
      </fieldset>
      <div className="mb-6 space-y-4 border-b border-border pb-6">
        {initialCreative ? (
          <h3 className="font-semibold">Artwork</h3>
        ) : (
          <label className="flex cursor-pointer items-center gap-2 font-semibold">
            <input type="checkbox" className="cursor-pointer accent-primary" checked={includeCreative} disabled={save.disabled || uploading}
              onChange={event => setIncludeCreative(event.target.checked)} />
            Add artwork
          </label>
        )}
        <div className={includeCreative ? "" : "hidden"}>
          <CreativeFields
            value={creative}
            disabled={save.disabled || !includeCreative}
            imagePending={pendingImages.image}
            onChange={setCreative}
            onPendingImage={(field, pending) => setPendingImages(previous => ({ ...previous, [field]: pending }))}
          />
        </div>
      </div>
      <fieldset disabled={save.disabled} className="min-w-0 space-y-6">
        <fieldset className="grid gap-3 sm:grid-cols-2">
          <legend className="mb-3 font-semibold">Placements</legend>
          {placements.map(placement => {
            const booking = draft.bookings.find(item => item.placementId === placement.id)

            return (
              <div key={placement.id} className={`flex items-center gap-3 rounded-card border p-3 ${booking ? "border-primary bg-surface" : "border-border"}`}>
                <label className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    className="cursor-pointer accent-primary"
                    checked={!!booking}
                    onChange={event => {
                      const bookings = event.target.checked
                        ? [...draft.bookings, { placementId: placement.id, share: 50 }]
                        : draft.bookings.filter(item => item.placementId !== placement.id)
                      change("bookings", bookings)
                    }}
                  />
                  {placement.name}
                </label>
                {booking && (
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      aria-label={`${placement.name} share`}
                      type="number"
                      min={1}
                      max={100}
                      className="w-16 rounded-input border border-border bg-surface px-2 py-1.5"
                      value={booking.share}
                      onChange={event => {
                        const share = Number(event.target.value)
                        const bookings = draft.bookings.map(item => item.placementId === placement.id ? { ...item, share } : item)
                        change("bookings", bookings)
                      }}
                    />
                    %
                  </label>
                )}
              </div>
            )
          })}
        </fieldset>
        <div className="grid gap-5 sm:grid-cols-2">
          <label className="text-sm font-medium">
            Start (local time)
            <input type="datetime-local" className={controlClass} value={utcToDatetimeLocalValue(draft.startAt)} onChange={event => changeDate("startAt", event.target.value)} />
          </label>
          <label className="text-sm font-medium">
            End (local time)
            <input type="datetime-local" className={controlClass} value={utcToDatetimeLocalValue(draft.endAt)} onChange={event => changeDate("endAt", event.target.value)} />
          </label>
          <label className="text-sm font-medium">
            Booking status
            <select className={controlClass} value={draft.state} onChange={event => change("state", event.target.value as SponsorCampaign["state"])}>
              <option value="draft">Draft</option>
              <option value="reserved">Reserved</option>
              <option value="booked">Booked</option>
              <option value="paused">Paused</option>
              <option value="cancelled">Cancelled</option>
            </select>
          </label>
          {draft.state === "reserved" && (
            <label className="text-sm font-medium">
              Confirm by
              <input type="datetime-local" className={controlClass} value={draft.confirmBy ? utcToDatetimeLocalValue(draft.confirmBy) : ""} onChange={event => changeDate("confirmBy", event.target.value)} />
            </label>
          )}
        </div>
        <details>
          <summary className="cursor-pointer font-medium">Targeting and exclusivity</summary>
          <div className="mt-4 grid gap-5 sm:grid-cols-2">
            <fieldset>
              <legend className="mb-2 text-sm font-medium">Page types</legend>
              <div className="flex flex-wrap gap-4">
                {[["home", "Home"], ["post", "Posts"], ["page", "Pages"]].map(([value, label]) => (
                  <label key={value} className="flex cursor-pointer items-center gap-2 text-sm">
                    <input type="checkbox" className="cursor-pointer accent-primary" checked={draft.pageTypes.includes(value)}
                      onChange={event => change("pageTypes", event.target.checked ? [...draft.pageTypes, value] : draft.pageTypes.filter(item => item !== value))} />
                    {label}
                  </label>
                ))}
              </div>
            </fieldset>
            <label className="text-sm font-medium">
              Language
              <select className={controlClass} value={draft.languages[0] || ""} onChange={event => change("languages", event.target.value ? [event.target.value] : [])}>
                <option value="">All languages</option>
                <option value="en">English</option>
                <option value="ru">Russian</option>
              </select>
            </label>
            <Input field="countries" label="Country codes (empty for all)" placeholder="AU, NZ" value={countries} onChange={setCountries} />
            <Input field="category" label="Advertiser category" value={draft.category} onChange={value => change("category", value)} />
          </div>
          <label className="flex cursor-pointer items-center gap-2 text-sm">
            <input type="checkbox" className="cursor-pointer accent-primary" checked={draft.exclusive} onChange={event => change("exclusive", event.target.checked)} />
            Exclusive category within the booked scope
          </label>
        </details>
        <details>
          <summary className="cursor-pointer font-medium">Payment and notes</summary>
          <div className="mt-4 grid gap-5 sm:grid-cols-3">
            <Input field="amountMinor" label="Booking amount" type="number" value={String(draft.amountMinor / 100)} onChange={value => change("amountMinor", Math.round(Number(value) * 100))} />
            <Input field="currency" label="Currency" value={draft.currency} onChange={value => change("currency", value.toUpperCase())} />
            <label className="text-sm font-medium">
              Payment
              <select className={controlClass} value={draft.paymentStatus} onChange={event => change("paymentStatus", event.target.value as SponsorCampaign["paymentStatus"])}>
                <option value="unpaid">Unpaid</option>
                <option value="partial">Partly paid</option>
                <option value="paid">Paid</option>
              </select>
            </label>
          </div>
          <TextArea field="notes" label="Booking notes" value={draft.notes} onChange={value => change("notes", value)} />
        </details>
      </fieldset>
      <div className="mt-5 flex justify-end">
        <SponsorSaveButton busy={save.busy} disabled={uploading || !!conflict} uncertain={save.uncertain} onClick={() => {
          if (save.uncertain) {
            void save.retry()
            return
          }

          void save.submit({
            baseCampaign: base.campaign,
            baseCreative: base.creative,
            campaign: { ...draft, countries: countries.toUpperCase().split(/[\s,]+/).filter(Boolean) },
            creative: includeCreative ? creative : null,
          })
        }} />
      </div>
    </SponsorEditor>
  )
}
