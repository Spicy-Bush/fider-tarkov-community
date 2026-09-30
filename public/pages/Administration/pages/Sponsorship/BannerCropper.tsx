import React, { useId, useRef, useState } from "react"
import { Button, Modal } from "@fider/components"
import { SponsorBannerImage, defaultBannerCrop } from "@fider/components/sponsorship/SponsorBannerImage"
import { SponsorImageCrop } from "@fider/models/sponsorBooking"
import * as notify from "@fider/services/notify"

export function BannerCropper({ imageKey, crop, onApply, onClose }: {
  imageKey: string
  crop?: SponsorImageCrop
  onApply: (crop: SponsorImageCrop) => void
  onClose: () => void
}) {
  const [draft, setDraft] = useState(crop ?? defaultBannerCrop)
  const [device, setDevice] = useState("desktop")
  const [status, setStatus] = useState<"loading" | "ready" | "failed">("loading")
  const [attempt, setAttempt] = useState(0)
  const image = useRef<HTMLImageElement>(null)
  const titleID = useId()
  const drag = useRef<{ x: number; y: number; crop: SponsorImageCrop }>()
  const bounded = (value: number) => Math.max(0, Math.min(100, value))

  return (
    <Modal.Window isOpen onClose={onClose} labelledBy={titleID} manageHistory={false} size="large" center={false}>
      <Modal.Header><span id={titleID}>Crop banner</span></Modal.Header>
      <Modal.Content>
        <div className="mb-4 flex gap-2">
          <Button variant={device === "desktop" ? "primary" : "secondary"} onClick={() => setDevice("desktop")}>Desktop</Button>
          <Button variant={device === "mobile" ? "primary" : "secondary"} onClick={() => setDevice("mobile")}>Mobile</Button>
        </div>
        <button
          type="button"
          aria-label="Crop position. Drag the image or use the arrow keys."
          disabled={status !== "ready"}
          className="mx-auto block max-w-full touch-none overflow-hidden rounded-card border border-border bg-surface p-0 text-left cursor-grab active:cursor-grabbing focus-visible:outline-2 focus-visible:outline-primary"
          style={{ width: device === "mobile" ? 390 : "100%" }}
          onPointerDown={event => {
            if (event.button !== 0) return

            drag.current = { x: event.clientX, y: event.clientY, crop: draft }
            event.currentTarget.setPointerCapture(event.pointerId)
          }}
          onPointerMove={event => {
            if (!drag.current) return

            const source = image.current!
            const width = source.clientWidth
            const height = source.clientHeight
            const scale = Math.max(width / source.naturalWidth, height / source.naturalHeight) * drag.current.crop.zoom
            const overflowX = source.naturalWidth * scale - width
            const overflowY = source.naturalHeight * scale - height

            setDraft({
              ...drag.current.crop,
              x: overflowX > 0 ? bounded(drag.current.crop.x - (event.clientX - drag.current.x) * 100 / overflowX) : 50,
              y: overflowY > 0 ? bounded(drag.current.crop.y - (event.clientY - drag.current.y) * 100 / overflowY) : 50,
            })
          }}
          onLostPointerCapture={() => { drag.current = undefined }}
          onKeyDown={event => {
            const step = event.shiftKey ? 10 : 2
            const horizontal = event.key === "ArrowLeft" ? -step : event.key === "ArrowRight" ? step : 0
            const vertical = event.key === "ArrowUp" ? -step : event.key === "ArrowDown" ? step : 0
            if (!horizontal && !vertical) return

            event.preventDefault()
            setDraft(previous => ({ ...previous, x: bounded(previous.x + horizontal), y: bounded(previous.y + vertical) }))
          }}
        >
          <SponsorBannerImage
            key={attempt}
            imageKey={imageKey}
            alt="Banner crop preview"
            device={device}
            crop={draft}
            imageRef={image}
            onLoad={() => setStatus("ready")}
            onError={() => {
              setStatus("failed")
              notify.error("Could not load the image for cropping.")
            }}
          />
        </button>
        {status === "failed" && (
          <Button onClick={() => {
            setStatus("loading")
            setAttempt(previous => previous + 1)
          }}>
            Retry image
          </Button>
        )}
        <label className="mt-5 flex items-center gap-4 text-sm font-medium">
          Zoom
          <input
            type="range" min={1} max={3} step={0.01} value={draft.zoom}
            disabled={status !== "ready"}
            className="min-w-0 flex-1 cursor-pointer accent-primary"
            onChange={event => setDraft(previous => ({ ...previous, zoom: Number(event.target.value) }))}
          />
        </label>
      </Modal.Content>
      <Modal.Footer>
        <div className="flex flex-wrap justify-end gap-2">
          <Button onClick={() => setDraft(defaultBannerCrop)}>Reset crop</Button>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" disabled={status !== "ready"} onClick={() => {
            onApply(draft)
            onClose()
          }}>
            Apply crop
          </Button>
        </div>
      </Modal.Footer>
    </Modal.Window>
  )
}
