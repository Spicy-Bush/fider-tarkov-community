import React, { useEffect, useRef, useState } from "react"
import { Button, Input, Modal, Moment, TextArea } from "@fider/components"
import { Tabs, TabPanels } from "@fider/components/common/Tabs"
import { PageConfig } from "@fider/components/layouts"
import { SponsorCampaign, SponsorCampaignSave, SponsorCreative, SponsorExclusion, SponsorManagement, SponsorSaveConflict } from "@fider/models/sponsorBooking"
import { deleteSponsorCampaign, deleteSponsorCreative, saveSponsorCampaign, saveSponsorExclusion } from "@fider/services/actions/sponsorBooking"
import { Fider, uploadedImageURL } from "@fider/services"
import { BookingEditor, newSponsorCampaign } from "./Sponsorship/BookingEditor"
import { newSponsorCreative } from "./Sponsorship/CreativeEditor"
import { Placements } from "./Sponsorship/Placements"
import { BookingReport } from "./Sponsorship/BookingReport"
import { Packages } from "./Sponsorship/Packages"
import { controlClass, SponsorSaveButton } from "./Sponsorship/SponsorControls"
import { useSponsorSave } from "./Sponsorship/useSponsorSave"
import { SponsorshipPackage } from "@fider/models"
import { SponsorPreview } from "./Sponsorship/SponsorPreview"
import * as notify from "@fider/services/notify"

export const pageConfig: PageConfig = {
  title: "Sponsorship",
  sidebarItem: "sponsorship",
  layoutVariant: "fullWidth",
}

const tabs = [
  { value: "bookings", label: "Bookings" },
  { value: "placements", label: "Placements" },
  { value: "exclusions", label: "Exclusions" },
  { value: "packages", label: "Packages" },
] as const

const tabKeys = tabs.map(tab => tab.value)
type SponsorTab = typeof tabKeys[number]

type BookingView =
  | { kind: "list" }
  | { kind: "campaign"; id: number }
  | { kind: "edit"; campaign: SponsorCampaign; creative?: SponsorCreative; conflict?: SponsorSaveConflict }

type DeleteTarget = { kind: "campaign" | "artwork"; id: number; campaignID: number; name: string }

function Exclusions({ exclusions, onChanged }: { exclusions: SponsorExclusion[]; onChanged: (exclusions: SponsorExclusion[]) => void }) {
  const [draft, setDraft] = useState<SponsorExclusion>({ pageType: "post", id: 0, reason: "" })
  const save = useSponsorSave(
    (value: { exclusion: SponsorExclusion; excluded: boolean }, submissionId: string) =>
      saveSponsorExclusion(value.exclusion, value.excluded, submissionId),
    onChanged,
  )

  return (
    <section className="space-y-5">
      <fieldset disabled={save.disabled} className="grid gap-4 sm:grid-cols-2">
        <label className="text-sm font-medium">
          Content type
          <select className={controlClass} value={draft.pageType} onChange={event => setDraft({ ...draft, pageType: event.target.value as "post" | "page" })}>
            <option value="post">Post</option>
            <option value="page">Page</option>
          </select>
        </label>
        <Input field="id" label="Content ID" type="number" value={draft.id ? String(draft.id) : ""} onChange={value => setDraft({ ...draft, id: Number(value) })} />
        <TextArea field="reason" label="Reason" value={draft.reason} onChange={value => setDraft({ ...draft, reason: value })} />
      </fieldset>
      <SponsorSaveButton
        busy={save.busy && save.pending?.excluded === true}
        uncertain={save.uncertain && save.pending?.excluded === true}
        disabled={save.pending?.excluded === false}
        onClick={() => {
          if (save.uncertain) void save.retry()
          else void save.submit({ exclusion: draft, excluded: true })
        }}
      />
      {exclusions.map(exclusion => {
        const pending = save.pending?.excluded === false && save.pending.exclusion.id === exclusion.id && save.pending.exclusion.pageType === exclusion.pageType

        return (
          <div key={`${exclusion.pageType}:${exclusion.id}`} className="flex items-center justify-between gap-4 rounded-card border border-border p-4">
            <div>
              <p className="m-0 font-medium">{exclusion.pageType === "post" ? "Post" : "Page"} #{exclusion.id}</p>
              <p className="mb-0 mt-1 text-sm text-muted">{exclusion.reason}</p>
            </div>
            <Button disabled={save.busy || (save.uncertain && !pending)} onClick={() => {
              if (pending) void save.retry()
              else void save.submit({ exclusion, excluded: false })
            }}>
              {pending && save.uncertain ? "Retry removal" : "Remove exclusion"}
            </Button>
          </div>
        )
      })}
    </section>
  )
}

export default function ManageSponsorshipPage({ management, packages, edit }: { management: SponsorManagement; packages: SponsorshipPackage[]; edit: boolean }) {
  const [data, setData] = useState(management)
  const [tab, setTab] = useState<SponsorTab>("bookings")
  const [view, setView] = useState<BookingView>(() => {
    if (!management.browse.campaignId) return { kind: "list" }
    if (edit) return { kind: "edit", campaign: management.campaigns[0], creative: management.creatives[0] }
    return { kind: "campaign", id: management.browse.campaignId }
  })
  const workspace = useRef<HTMLDivElement>(null)
  const [previewID, setPreviewID] = useState<number>()
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget>()
  const selected = view.kind === "campaign" ? data.campaigns.find(campaign => campaign.id === view.id) : undefined
  const preview = data.creatives.find(item => item.id === previewID)
  const previewCampaign = preview ? data.campaigns.find(campaign => campaign.id === preview.campaignId)! : undefined
  const browse = management.browse
  const bookingURL = (campaignID = 0, page = browse.page, artworkPage = 1) => {
    const params = new URLSearchParams({ page: String(page) })
    if (browse.search) params.set("search", browse.search)
    if (campaignID) params.set("campaign", String(campaignID))
    if (artworkPage > 1) params.set("artworkPage", String(artworkPage))
    return "/admin/sponsorship?" + params
  }

  useEffect(() => {
    workspace.current?.focus({ preventScroll: true })
    window.scrollTo({ top: 0 })
  }, [view])

  const editCampaign = (campaign: SponsorCampaign, creative?: SponsorCreative) => {
    const latest = creative ?? data.creatives
      .filter(item => item.campaignId === campaign.id)
      .sort((left, right) => right.id - left.id)[0]
    setView({ kind: "edit", campaign, creative: latest })
  }

  const refreshCampaign = (value: SponsorCampaignSave) => {
    setData(previous => ({
      ...previous,
      campaigns: [...previous.campaigns.filter(item => item.id !== value.campaign.id), value.campaign],
      creatives: value.creative
        ? [...previous.creatives.filter(item => item.id !== value.creative!.id), value.creative]
        : previous.creatives,
    }))
  }

  const savedCampaign = (value: SponsorCampaignSave) => {
    window.location.assign(bookingURL(value.campaign.id))
  }

  const changeState = useSponsorSave(saveSponsorCampaign, result => {
    if (result.kind === "saved") {
      savedCampaign(result.saved)
    } else {
      refreshCampaign(result.saved)
      for (const problem of result.problems) notify.error(problem)
      setView({ kind: "edit", campaign: result.saved.campaign, creative: result.saved.creative ?? undefined, conflict: result })
    }
  })

  const remove = useSponsorSave(async (target: DeleteTarget) => {
    const result = target.kind === "campaign"
      ? await deleteSponsorCampaign(target.id)
      : await deleteSponsorCreative(target.id)

    return result.ok ? { ok: true as const, data: target } : result
  }, target => {
    window.location.assign(bookingURL(target.kind === "campaign" ? 0 : target.campaignID, 1))
  })

  const repeat = (campaign: SponsorCampaign) => {
    const start = new Date()
    const duration = new Date(campaign.endAt).getTime() - new Date(campaign.startAt).getTime()
    setView({
      kind: "edit",
      campaign: {
        ...campaign,
        id: 0,
        revision: 0,
        state: "draft",
        startAt: start.toISOString(),
        endAt: new Date(start.getTime() + duration).toISOString(),
        confirmBy: null,
        paymentStatus: "unpaid",
      },
    })
  }

  return (
    <div className="mx-auto max-w-[1400px] space-y-6">
      <Tabs tabs={tabs} activeTab={tab} onChange={setTab} className="overflow-x-auto" listClassName="flex min-w-max border-b border-surface-alt" />
      <TabPanels keys={tabKeys} activeKey={tab} keepMounted>
        {active => {
          if (active === "placements") {
            return <Placements placements={data.placements} onSaved={placement => setData(previous => ({
              ...previous,
              placements: previous.placements.map(item => item.id === placement.id ? placement : item),
            }))} />
          }

          if (active === "packages") return <Packages initial={packages} placements={data.placements} />

          if (active === "exclusions") {
            return <Exclusions exclusions={data.exclusions} onChanged={exclusions => setData(previous => ({ ...previous, exclusions }))} />
          }

          return (
            <div ref={workspace} tabIndex={-1} className="space-y-6 outline-none">
              {view.kind === "edit" && (
                <BookingEditor
                  key={`${view.campaign.id}:${view.creative?.id ?? 0}`}
                  initial={view.campaign}
                  initialCreative={view.creative}
                  initialConflict={view.conflict}
                  placements={data.placements}
                  onSaved={savedCampaign}
                  onRefreshed={refreshCampaign}
                  onClose={() => setView(view.campaign.id ? { kind: "campaign", id: view.campaign.id } : { kind: "list" })}
                />
              )}
              {changeState.uncertain && selected && (
                <Button onClick={() => changeState.retry()} loading={changeState.busy}>Retry status change</Button>
              )}
              {view.kind === "list" && (
                <>
                  <div className="flex flex-wrap items-end justify-between gap-4">
                    <Button variant="primary" onClick={() => setView({ kind: "edit", campaign: newSponsorCampaign() })}>New campaign</Button>
                    <form action="/admin/sponsorship" method="get" className="flex items-center gap-2">
                      <input name="search" aria-label="Search bookings" placeholder="Search bookings" defaultValue={browse.search} maxLength={100} className={controlClass} />
                      <Button type="submit">Search</Button>
                    </form>
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    {data.campaigns.map(campaign => (
                      <a key={campaign.id} href={bookingURL(campaign.id)}
                        className="cursor-pointer rounded-panel border border-border p-4 text-left transition-colors hover:bg-elevated">
                        <span className="block font-semibold">{campaign.name}</span>
                        <span className="mt-1 block text-sm text-muted">{campaign.advertiser}</span>
                        <span className="mt-2 block text-sm">{campaign.state}</span>
                      </a>
                    ))}
                  </div>
                  {data.campaigns.length === 0 && <p className="text-muted">No bookings found.</p>}
                  <div className="flex items-center gap-3">
                    <Button href={browse.page > 1 ? bookingURL(0, browse.page - 1) : undefined} disabled={browse.page === 1}>Previous</Button>
                    <span className="text-sm text-muted">Page {browse.page}</span>
                    <Button href={data.nextPage ? bookingURL(0, browse.page + 1) : undefined} disabled={!data.nextPage}>Next</Button>
                  </div>
                </>
              )}
              {selected && (
                <section className="space-y-5">
                  <Button disabled={changeState.disabled} href={bookingURL()}>Back to bookings</Button>
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <h2 className="mb-1 text-xl font-semibold">{selected.name}</h2>
                      <p className="m-0 text-sm text-muted">
                        Start: <Moment locale={Fider.currentLocale} date={selected.startAt} />
                        {", end: "}<Moment locale={Fider.currentLocale} date={selected.endAt} />
                      </p>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Button variant="primary" disabled={changeState.disabled} href={bookingURL(selected.id) + "&edit=1"}>Edit campaign</Button>
                      <Button disabled={changeState.disabled} onClick={() => repeat(selected)}>Repeat campaign</Button>
                      <Button variant="danger" disabled={changeState.disabled} onClick={() => setDeleteTarget({
                        kind: "campaign", id: selected.id, campaignID: selected.id, name: selected.name,
                      })}>
                        Delete campaign
                      </Button>
                      {(selected.state === "booked" || selected.state === "paused") && (
                        <Button disabled={changeState.disabled} onClick={() => changeState.submit({
                          baseCampaign: selected,
                          campaign: { ...selected, state: selected.state === "booked" ? "paused" : "booked" },
                          creative: null,
                        })}>{selected.state === "booked" ? "Pause" : "Resume"}</Button>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-wrap gap-3">
                    {selected.bookings.map(item => <span key={item.placementId} className="text-sm">{data.placements.find(placement => placement.id === item.placementId)!.name}: {item.share}%</span>)}
                  </div>
                  <Button disabled={changeState.disabled} onClick={() => editCampaign(selected, newSponsorCreative(selected.id))}>Add artwork</Button>
                  <div className="space-y-3">
                    {data.creatives.filter(item => item.campaignId === selected.id).map(item => (
                      <article key={item.id} className="flex flex-wrap items-center justify-between gap-3 rounded-card border border-border p-4">
                        <div className="flex min-w-0 items-center gap-4">
                          {item.imageKey && <img src={uploadedImageURL(item.imageKey, 200)} alt="" className="h-20 w-32 object-contain" />}
                          <div>
                            <p className="m-0 font-semibold">{item.headline || `Artwork #${item.id}`}</p>
                            <p className="mb-0 mt-1 text-sm text-muted">{item.language || "all languages"}, {item.device || "all devices"}: {item.state}</p>
                            {item.reviewReason && <p className="mb-0 mt-2 text-sm">{item.reviewReason}</p>}
                          </div>
                        </div>
                        <div className="flex flex-wrap gap-2">
                          <Button onClick={() => setPreviewID(item.id)}>Preview</Button>
                          <Button disabled={changeState.disabled} onClick={() => editCampaign(selected, item)}>Edit artwork</Button>
                          <Button variant="danger" disabled={changeState.disabled} onClick={() => setDeleteTarget({
                            kind: "artwork", id: item.id, campaignID: selected.id, name: item.headline || `Artwork #${item.id}`,
                          })}>
                            Delete artwork
                          </Button>
                        </div>
                      </article>
                    ))}
                  </div>
                  {(browse.artworkPage > 1 || data.nextArtwork) && (
                    <div className="flex items-center gap-3" aria-label="Artwork pages">
                      <Button href={browse.artworkPage > 1 ? bookingURL(selected.id, browse.page, browse.artworkPage - 1) : undefined} disabled={browse.artworkPage === 1}>Previous artwork</Button>
                      <span className="text-sm text-muted">Page {browse.artworkPage}</span>
                      <Button href={data.nextArtwork ? bookingURL(selected.id, browse.page, browse.artworkPage + 1) : undefined} disabled={!data.nextArtwork}>Next artwork</Button>
                    </div>
                  )}
                  <BookingReport key={`report:${selected.id}:${selected.revision}`} campaign={selected} placements={data.placements} />
                </section>
              )}
            </div>
          )
        }}
      </TabPanels>
      <Modal.Window
        isOpen={!!deleteTarget}
        canClose={!remove.disabled}
        onClose={() => { if (!remove.disabled) setDeleteTarget(undefined) }}
        labelledBy="delete-sponsor-title"
        manageHistory={false}
        size="small"
      >
        {deleteTarget && (
          <>
            <Modal.Header>
              <h2 id="delete-sponsor-title" className="m-0 text-title">Delete {deleteTarget.kind}</h2>
            </Modal.Header>
            <Modal.Content>
              <p className="m-0">
                {deleteTarget.kind === "campaign"
                  ? `Delete ${deleteTarget.name} and all its artwork?`
                  : `Delete ${deleteTarget.name}?`}
                {" This cannot be undone."}
              </p>
            </Modal.Content>
            <Modal.Footer>
              <div className="flex justify-end gap-2">
                <Button disabled={remove.disabled} onClick={() => setDeleteTarget(undefined)}>Cancel</Button>
                <Button variant="danger" loading={remove.busy} onClick={() => {
                  if (remove.uncertain) void remove.retry()
                  else void remove.submit(deleteTarget)
                }}>
                  {remove.uncertain ? "Retry delete" : "Delete"}
                </Button>
              </div>
            </Modal.Footer>
          </>
        )}
      </Modal.Window>
      <Modal.Window isOpen={!!preview} onClose={() => setPreviewID(undefined)} labelledBy="saved-sponsor-preview" manageHistory={false} size="large" center={false}>
        {preview && (
          <>
            <Modal.Header>
              <h2 id="saved-sponsor-preview" className="text-title m-0">Preview</h2>
            </Modal.Header>
            <Modal.Content>
              <SponsorPreview
                key={preview.id}
                creative={preview}
                advertiser={previewCampaign!.advertiser}
                bookings={previewCampaign!.bookings}
                placements={data.placements}
              />
            </Modal.Content>
            <Modal.Footer>
              <Button onClick={() => setPreviewID(undefined)}>Close preview</Button>
            </Modal.Footer>
          </>
        )}
      </Modal.Window>
    </div>
  )
}
