import React from "react"
import { FeedNativeAd } from "./FeedNativeAd"
import { useSponsorshipAd, useSponsorshipLoaded } from "./SponsorshipProvider"
import { SPONSORSHIP_SLOT_SPECS, SponsorshipSlot } from "@fider/models"

interface AdSlotProps {
  slot: SponsorshipSlot
  className?: string
}

/**
 * House ad for slot from SponsorshipProvider.
 * Requires non-empty advertiser — empty means do not render (no bare "Sponsored").
 */
export const AdSlot: React.FC<AdSlotProps> = ({ slot, className }) => {
  const loaded = useSponsorshipLoaded()
  const campaign = useSponsorshipAd(slot)

  if (!loaded || !campaign) {
    return null
  }

  const advertiser = (campaign.advertiser || "").trim()
  if (!advertiser) {
    return null
  }

  if (slot === "feed_native") {
    return <FeedNativeAd campaign={campaign} className={className} />
  }

  const spec = SPONSORSHIP_SLOT_SPECS[slot]
  const hasImage = Boolean(campaign.creativeImageUrl)
  const hasHtml = Boolean(campaign.creativeHtml)
  const disclosure = `Sponsored · ${advertiser}`

  return (
    <a
      href={campaign.clickPath}
      className={className || "block my-3 no-underline"}
      rel="sponsored noopener"
      target="_blank"
      aria-label={disclosure}
    >
      <div className="text-[10px] uppercase tracking-wide text-muted mb-1">{disclosure}</div>
      {hasImage && (
        <div className={spec.frameClassName}>
          <img
            src={campaign.creativeImageUrl!}
            alt={advertiser}
            className={spec.imgClassName}
            loading="lazy"
          />
        </div>
      )}
      {hasHtml && (
        <div className="sponsorship-html mt-2" dangerouslySetInnerHTML={{ __html: campaign.creativeHtml! }} />
      )}
      {!hasImage && !hasHtml && <span className="text-sm text-muted">{advertiser}</span>}
    </a>
  )
}
