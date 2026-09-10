import React from "react"
import { FeedNativeAd } from "./FeedNativeAd"
import { useSponsorshipAd, useSponsorshipLoaded } from "./SponsorshipProvider"
import { SPONSORSHIP_SLOT_SPECS, SponsorshipSlot } from "@fider/models"

interface AdSlotProps {
  slot: SponsorshipSlot
  className?: string
}

/**
 * Renders a house ad for slot from SponsorshipProvider (no per-slot fetch).
 * Must be under a SponsorshipProvider that requested this slot.
 * Image + HTML both render when present. No swipe-mode wiring (by design).
 */
export const AdSlot: React.FC<AdSlotProps> = ({ slot, className }) => {
  const loaded = useSponsorshipLoaded()
  const campaign = useSponsorshipAd(slot)

  if (!loaded || !campaign) {
    return null
  }

  if (slot === "feed_native") {
    return <FeedNativeAd campaign={campaign} className={className} />
  }

  const spec = SPONSORSHIP_SLOT_SPECS[slot]
  const hasImage = Boolean(campaign.creativeImageUrl)
  const hasHtml = Boolean(campaign.creativeHtml)

  return (
    <a
      href={campaign.clickPath}
      className={className || "block my-3 no-underline"}
      rel="sponsored noopener"
      target="_blank"
      aria-label={campaign.name}
    >
      {hasImage && (
        <div className={spec.frameClassName}>
          <img
            src={campaign.creativeImageUrl!}
            alt={campaign.name}
            className={spec.imgClassName}
            loading="lazy"
          />
        </div>
      )}
      {hasHtml && (
        <div className="sponsorship-html mt-2" dangerouslySetInnerHTML={{ __html: campaign.creativeHtml! }} />
      )}
      {!hasImage && !hasHtml && <span className="text-sm text-muted">{campaign.name}</span>}
      <div className="text-[10px] uppercase tracking-wide text-muted mt-1">Sponsored</div>
    </a>
  )
}
