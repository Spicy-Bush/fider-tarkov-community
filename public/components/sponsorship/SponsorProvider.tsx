import React, { createContext, useContext, useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore } from "react"
import { SponsorArtwork, SponsorContext, SponsorOpportunity, SponsorPlacement, SponsorSelection } from "@fider/models/sponsorBooking"
import { allocateSponsors } from "@fider/services/actions/sponsorBooking"
import { Fider } from "@fider/services/fider"
import { RequestError } from "@fider/services/http"
import { SponsorCard } from "./SponsorCard"

type SlotState = { status: "pending" | "failed" } | { status: "ready"; selection: SponsorSelection }

export function createSponsorSelection(context: SponsorContext, placements: SponsorPlacement[], preview = false) {
  let slots: Readonly<Record<string, SlotState>> = {}
  const listeners = new Set<() => void>()
  const queued = new Map<string, SponsorOpportunity>()
  let scheduled = false

  const publish = (changed: Record<string, SlotState>) => {
    slots = { ...slots, ...changed }
    for (const listener of listeners) listener()
  }

  const flush = async () => {
    while (queued.size > 0) {
      const batch = [...queued.values()].slice(0, 32)
      for (const item of batch) queued.delete(item.instanceId)

      const changed: Record<string, SlotState> = {}
      try {
        const result = await allocateSponsors(context, batch)
        for (const item of batch) {
          if (result.ok && !result.data[item.instanceId]) {
            throw new Error(`Missing sponsorship selection for ${item.instanceId}`)
          }

          changed[item.instanceId] = result.ok
            ? { status: "ready", selection: result.data[item.instanceId] }
            : { status: "failed" }
        }
      } catch (cause) {
        if (!(cause instanceof RequestError)) throw cause

        for (const item of batch) changed[item.instanceId] = { status: "failed" }
      }

      publish(changed)
    }

    scheduled = false
  }

  return {
    placements,
    preview,
    previewElement: null as HTMLDivElement | null,
    snapshot: () => slots,
    subscribe: (listener: () => void) => {
      listeners.add(listener)
      return () => { listeners.delete(listener) }
    },
    request: (opportunity: SponsorOpportunity) => {
      if (preview) return

      if (slots[opportunity.instanceId]) return

      publish({ [opportunity.instanceId]: { status: "pending" } })
      queued.set(opportunity.instanceId, opportunity)
      if (!scheduled) {
        scheduled = true
        queueMicrotask(() => { void flush() })
      }
    },
  }
}

type SelectionOwner = ReturnType<typeof createSponsorSelection>
const SponsorOwner = createContext<SelectionOwner | null>(null)

export function SponsorProvider({ children }: { children: React.ReactNode }) {
  const data = useSyncExternalStore(Fider.session.subscribe, Fider.session.getSnapshot, Fider.session.getSnapshot)
  const [selection, setSelection] = useState<{ key: string; owner: SelectionOwner }>()
  const [device, setDevice] = useState<string>()
  const pageType = data.page === "Home/Home.page" ? "home" : data.page === "ShowPost/ShowPost.page" ? "post" : data.page === "Page/ViewPage.page" ? "page" : ""
  const id = pageType === "post" ? data.props.post.id : pageType === "page" ? data.props.page.id : 0
  const language = Fider.currentLocale.startsWith("ru") ? "ru" : "en"
  const preview = data.props.sponsorPreview === true
  const key = JSON.stringify([data.tenant.id, pageType, id, language, device, preview, data.props.sponsorPlacements])

  useEffect(() => {
    const viewport = window.matchMedia("(min-width: 1024px)")
    const resized = () => setDevice(viewport.matches ? "desktop" : "mobile")
    resized()
    viewport.addEventListener("change", resized)

    return () => viewport.removeEventListener("change", resized)
  }, [])

  useEffect(() => {
    if (!pageType || !device) {
      setSelection(undefined)
      return
    }

    const context = { pageType, id, language, device }
    const previewPlacement = new URLSearchParams(window.location.search).get("placement")
    const placements = (data.props.sponsorPlacements as SponsorPlacement[] || []).filter(p =>
      (preview ? p.id === previewPlacement : p.enabled) &&
      p.device === device && (p.pageType === "all" || p.pageType === pageType)
    )
    setSelection({ key, owner: createSponsorSelection(context, placements, preview) })
  }, [key])

  return (
    <SponsorOwner.Provider value={selection?.key === key ? selection.owner : null}>
      {children}
    </SponsorOwner.Provider>
  )
}

export function useSponsorPlacement(position: string): SponsorPlacement | undefined {
  return useContext(SponsorOwner)?.placements.find(p => p.position === position)
}

export function useSponsorPreview(): boolean {
  return useContext(SponsorOwner)?.preview === true
}

function SelectedSponsor({ owner, placement, instance, reservedHeight }: { owner: SelectionOwner; placement: SponsorPlacement; instance: string; reservedHeight?: number }) {
  const slots = useSyncExternalStore(owner.subscribe, owner.snapshot, owner.snapshot)
  const element = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) {
        owner.request({ instanceId: instance, placementId: placement.id })
        observer.disconnect()
      }
    }, { rootMargin: "600px" })
    observer.observe(element.current!)
    return () => observer.disconnect()
  }, [owner, instance, placement.id])

  const slot = slots[instance]
  const pending = !slot || slot.status === "pending"
  const empty = slot?.status === "failed" || (slot?.status === "ready" && slot.selection.kind === "none")

  return (
    <div
      ref={element}
      data-feed-slot={placement.position === "feed" ? instance : undefined}
      hidden={empty}
      style={pending && reservedHeight ? { height: reservedHeight } : undefined}
    >
      {slot?.status === "ready" && <SponsorCard selection={slot.selection} />}
    </div>
  )
}

export function SponsorSpot({ position, instance, reservedHeight, unavailable = false }: { position: string; instance?: string; reservedHeight?: number; unavailable?: boolean }) {
  const owner = useContext(SponsorOwner)
  const placement = owner?.placements.find(p => p.position === position)
  if (!owner || !placement) return null

  if (owner.preview) return <PreviewSponsor owner={owner} placement={placement} unavailable={unavailable} />
  if (unavailable) return null

  return <SelectedSponsor owner={owner} placement={placement} instance={instance ?? placement.id} reservedHeight={reservedHeight} />
}

function PreviewSponsor({ owner, placement, unavailable }: { owner: SelectionOwner; placement: SponsorPlacement; unavailable: boolean }) {
  const element = useRef<HTMLDivElement>(null)
  const [artwork, setArtwork] = useState<{ creative: SponsorArtwork; advertiser: string }>()

  useEffect(() => {
    if (owner.previewElement) return

    const target = element.current!
    owner.previewElement = target
    document.body.inert = true

    if (unavailable) {
      window.parent.postMessage({ type: "sponsor-preview-unavailable", placementID: placement.id }, window.location.origin)
      return () => {
        owner.previewElement = null
        document.body.inert = false
      }
    }

    const receive = (event: MessageEvent) => {
      if (event.origin !== window.location.origin || event.source !== window.parent) return
      if (event.data?.type !== "sponsor-preview-show" || event.data.placementID !== placement.id) return

      setArtwork({ creative: event.data.creative, advertiser: event.data.advertiser })
    }
    window.addEventListener("message", receive)
    window.parent.postMessage({ type: "sponsor-preview-ready", placementID: placement.id }, window.location.origin)

    return () => {
      owner.previewElement = null
      document.body.inert = false
      window.removeEventListener("message", receive)
    }
  }, [owner, placement.id, unavailable])

  useLayoutEffect(() => {
    if (!artwork) return

    const target = element.current!
    const reveal = () => {
      const bounds = target.getBoundingClientRect()
      window.scrollTo({ top: window.scrollY + bounds.top - (window.innerHeight - bounds.height) / 2 })
    }
    reveal()
    target.addEventListener("load", reveal, true)
    window.parent.postMessage({ type: "sponsor-preview-rendered", placementID: placement.id }, window.location.origin)

    return () => target.removeEventListener("load", reveal, true)
  }, [artwork, placement.id])

  return (
    <div ref={element} data-sponsor-position={placement.position}>
      {artwork && <SponsorCard preview selection={{ kind: "sponsor", placement, ...artwork, expiresAt: "", clickUrl: "" }} />}
    </div>
  )
}
