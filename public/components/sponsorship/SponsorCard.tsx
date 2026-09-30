import React, { useEffect, useState } from "react"
import { SponsorSelection } from "@fider/models/sponsorBooking"
import { Moment } from "@fider/components/common/Moment"
import { Fider } from "@fider/services/fider"
import { SponsorBannerImage } from "./SponsorBannerImage"
import { AdSenseSlot } from "./AdSenseSlot"

export function SponsorCard({ selection, preview = false }: { selection: SponsorSelection; preview?: boolean }) {
  const deadline = selection.kind === "sponsor" && !preview ? new Date(selection.expiresAt).getTime() : Infinity
  const [now, setNow] = useState(Date.now)

  useEffect(() => {
    if (!Number.isFinite(deadline) || now >= deadline) return

    const delay = Math.min(Math.max(0, deadline - Date.now()), 2_147_483_647)
    const timeout = window.setTimeout(() => setNow(Date.now()), delay)
    return () => window.clearTimeout(timeout)
  }, [deadline, now])

  if (selection.kind === "none" || now >= deadline) return null

  if (selection.kind === "adsense") {
    return preview ? null : <AdSenseSlot
      client={Fider.settings.googleAdSense || ""}
      slotId={selection.placement.adsenseSlotId || ""}
      placementId={selection.placement.id}
      format={selection.placement.position === "navigation" ? "horizontal" : "auto"}
    />
  }

  if (selection.kind === "kofi") {
    return (
      <aside className="my-3 rounded-card border border-border bg-elevated px-4 py-3" aria-label="Support the community">
        <a href="https://ko-fi.com/tarkovcommunity" target="_blank" rel="noopener noreferrer" className="cursor-pointer font-medium">
          Support Tarkov Community on Ko-fi
        </a>
      </aside>
    )
  }

  const { creative, advertiser, placement } = selection
  const banner = placement.position === "navigation"
  const textStrip = banner && !creative.imageKey
  const hasCopy = creative.logoKey || creative.headline || creative.description || creative.callToAction
  const imageURL = `/static/images/${creative.imageKey}`

  return (
    <aside className={`my-3 min-w-0 max-w-full ${banner ? "w-full" : creative.imageKey && !hasCopy ? "mx-auto w-fit" : ""}`} aria-label={`Advertisement from ${advertiser}`}>
      <span className="mb-1 block text-xs text-muted">Advertisement</span>
      <div className={creative.framed ? "overflow-hidden rounded-card border border-border bg-elevated" : ""}>
        <a
          href={preview ? undefined : selection.clickUrl}
          aria-label={creative.headline || advertiser}
          target="_blank"
          rel="sponsored noopener noreferrer"
          className={`block text-inherit no-underline ${preview ? "" : "cursor-pointer"}`}
        >
          {creative.imageKey && (banner ? (
            <SponsorBannerImage imageKey={creative.imageKey} alt={creative.headline || advertiser} device={placement.device} crop={creative.bannerCrop} />
          ) : (
            <picture>
              <source media="(min-width: 768px), (min-resolution: 2dppx)" srcSet={`${imageURL}?size=1500`} />
              <img
                src={`${imageURL}?size=512`}
                alt={creative.headline || advertiser}
                className="mx-auto block h-auto w-auto max-w-full"
                style={{ maxHeight: 320 }}
                loading="lazy"
              />
            </picture>
          ))}
          {hasCopy && (
            <div className={`${creative.framed ? "p-3" : "py-2"} ${textStrip ? "flex flex-wrap items-center gap-4" : "space-y-2"}`}>
              {creative.logoKey && (
                <img src={`/static/images/${creative.logoKey}?size=200`} alt="" className="h-12 w-20 shrink-0 object-contain" loading="lazy" />
              )}
              {(creative.headline || creative.description) && (
                <div className="min-w-0 flex-1">
                  {creative.headline && <p className="m-0 font-semibold wrap-anywhere">{creative.headline}</p>}
                  {creative.description && <p className="mb-0 mt-1 text-sm text-muted wrap-anywhere">{creative.description}</p>}
                </div>
              )}
              {creative.callToAction && <span className="inline-block font-medium text-primary">{creative.callToAction}</span>}
            </div>
          )}
        </a>
        {creative.offerCode && (
          <div className={`${creative.framed ? "px-3 pb-3" : "pb-2"} pt-2 text-sm`}>
            <span className="select-all rounded bg-surface px-2 py-1 font-mono">{creative.offerCode}</span>
            <p className="mb-0 mt-2 text-muted">{creative.offerTerms}</p>
            <p className="mb-0 mt-1 text-muted">Expires <Moment locale={Fider.currentLocale} date={creative.offerExpires!} /></p>
          </div>
        )}
      </div>
    </aside>
  )
}
