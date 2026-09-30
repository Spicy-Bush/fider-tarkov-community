import React, { useEffect, useId, useLayoutEffect, useRef, useState } from "react"
import { Button, Modal } from "@fider/components"
import { SponsorBooking, SponsorCreative, SponsorPlacement } from "@fider/models/sponsorBooking"
import * as notify from "@fider/services/notify"

interface SponsorPreviewProps {
  creative: SponsorCreative
  advertiser: string
  placements: SponsorPlacement[]
  bookings: SponsorBooking[]
}

type PreviewState =
  | { kind: "loading" | "ready" }
  | { kind: "failed" | "unavailable"; error: string }

function PreviewFrame({ creative, advertiser, placement, width, scale, retry }: {
  creative: SponsorCreative
  advertiser: string
  placement: SponsorPlacement
  width: number
  scale: number
  retry: () => void
}) {
  const frame = useRef<HTMLIFrameElement>(null)
  const [state, setState] = useState<PreviewState>({ kind: "loading" })
  const pageType = placement.pageType === "all" ? "home" : placement.pageType
  const visible = state.kind === "ready" || state.kind === "unavailable"
  const error = "error" in state ? state.error : undefined

  useEffect(() => {
    if (error) notify.error(error)
  }, [error])

  useEffect(() => {
    const show = () => frame.current?.contentWindow?.postMessage({
      type: "sponsor-preview-show", placementID: placement.id, creative, advertiser,
    }, window.location.origin)
    const receive = (event: MessageEvent) => {
      if (event.origin !== window.location.origin || event.source !== frame.current?.contentWindow) return
      if (event.data?.placementID !== placement.id) return

      if (event.data.type === "sponsor-preview-ready") show()
      if (event.data.type === "sponsor-preview-rendered") setState({ kind: "ready" })
      if (event.data.type === "sponsor-preview-unavailable") {
        setState({ kind: "unavailable", error: "The selected placement is unavailable in this preview." })
      }
    }
    window.addEventListener("message", receive)
    show()

    return () => window.removeEventListener("message", receive)
  }, [placement.id, creative, advertiser])

  return (
    <>
      {error && <Button className="mb-3" onClick={retry}>Retry preview</Button>}
      <div className="relative h-[650px] overflow-hidden rounded-panel border border-border bg-surface">
        {state.kind === "loading" && <span role="status" className="absolute inset-0 grid place-items-center text-muted">Loading preview...</span>}
        <iframe
          ref={frame}
          title={`${pageType === "home" ? "Home" : pageType === "post" ? "Post" : "Page"} preview`}
          src={`/admin/sponsorship/preview?pageType=${pageType}&placement=${placement.id}`}
          className="absolute left-1/2 top-0 border-0"
          style={{
            width,
            height: 650 / scale,
            transform: `translateX(-50%) scale(${scale})`,
            transformOrigin: "top center",
            visibility: visible ? "visible" : "hidden",
          }}
          onLoad={event => {
            const data = event.currentTarget.contentDocument?.getElementById("server-data")?.textContent
            if (!data || !JSON.parse(data).props?.sponsorPreview) {
              setState({ kind: "failed", error: "Could not load this preview. Check that a published page or post is available." })
            }
          }}
        />
      </div>
    </>
  )
}

export function SponsorPreview({ creative, advertiser, placements, bookings }: SponsorPreviewProps) {
  const [device, setDevice] = useState("desktop")
  const [placementID, setPlacementID] = useState<string>()
  const [attempt, setAttempt] = useState(0)
  const container = useRef<HTMLDivElement>(null)
  const [scale, setScale] = useState(1)
  const width = device === "desktop" ? 1280 : 390
  const choices = placements.filter(placement =>
    placement.device === device && bookings.some(booking => booking.placementId === placement.id)
  )
  const placement = choices.find(item => item.id === placementID) ?? choices[0]

  useLayoutEffect(() => {
    const measure = () => setScale(Math.min(1, container.current!.clientWidth / width))
    const observer = new ResizeObserver(measure)
    observer.observe(container.current!)
    measure()

    return () => observer.disconnect()
  }, [width])

  return (
    <div className="space-y-4">
      <div className="flex gap-2" aria-label="Preview device">
        {["desktop", "mobile"].map(value => (
          <Button key={value} variant={device === value ? "primary" : "secondary"} onClick={() => setDevice(value)}>
            {value === "desktop" ? "Desktop" : "Mobile"}
          </Button>
        ))}
      </div>
      {choices.length > 1 && (
        <div className="flex flex-wrap gap-2" aria-label="Preview placement">
          {choices.map(item => (
            <Button key={item.id} variant={item.id === placement.id ? "primary" : "secondary"} onClick={() => setPlacementID(item.id)}>
              {item.name}
            </Button>
          ))}
        </div>
      )}
      <div ref={container}>
        {placement ? (
          <PreviewFrame
            key={`${placement.id}:${placement.position}:${attempt}`}
            creative={creative}
            advertiser={advertiser}
            placement={placement}
            width={width}
            scale={scale}
            retry={() => setAttempt(previous => previous + 1)}
          />
        ) : (
          <div className="grid h-[650px] place-items-center rounded-panel border border-border text-muted">
            Choose a placement for this device.
          </div>
        )}
      </div>
    </div>
  )
}

export function SponsorEditor({ title, preview, disabled, onClose, children }: {
  title: string
  preview?: React.ReactNode
  disabled: boolean
  onClose: () => void
  children: React.ReactNode
}) {
  const [open, setOpen] = useState(false)
  const [docked, setDocked] = useState(false)
  const previewID = useId()
  const previewButton = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const query = window.matchMedia("(min-width: 1280px)")
    const changed = () => setDocked(query.matches)
    changed()
    query.addEventListener("change", changed)

    return () => query.removeEventListener("change", changed)
  }, [])

  const visible = open && !!preview
  const closePreview = () => {
    setOpen(false)
    previewButton.current?.querySelector("button")?.focus()
  }

  const panel = (
    <>
      <div className="mb-5 flex items-center justify-between gap-3">
        <h2 id={previewID} className="text-title m-0">Preview</h2>
        <Button onClick={closePreview}>Close preview</Button>
      </div>
      {preview}
    </>
  )

  return (
    <div className={visible && docked ? "grid grid-cols-2 items-start gap-6" : "mx-auto max-w-[900px]"}>
      <section className="min-w-0 rounded-panel border border-border bg-elevated p-4 sm:p-6">
        <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-title m-0">{title}</h2>
          <div className="flex gap-2">
            {preview && <div ref={previewButton}><Button onClick={() => setOpen(previous => !previous)}>Preview</Button></div>}
            <Button onClick={onClose} disabled={disabled}>Cancel</Button>
          </div>
        </div>
        {children}
      </section>
      {visible && (docked ? (
        <aside
          aria-labelledby={previewID}
          onKeyDown={event => {
            if (event.key === "Escape") closePreview()
          }}
          className="sticky top-6 max-h-[calc(100vh-3rem)] min-w-0 overflow-y-auto rounded-panel border border-border bg-surface p-5 animate-[slideInFromRight_180ms_var(--ease-out)] motion-reduce:animate-none"
        >
          {panel}
        </aside>
      ) : (
        <Modal.Window isOpen onClose={closePreview} labelledBy={previewID} manageHistory={false} size="large" center={false}>
          <Modal.Content>{panel}</Modal.Content>
        </Modal.Window>
      ))}
    </div>
  )
}
