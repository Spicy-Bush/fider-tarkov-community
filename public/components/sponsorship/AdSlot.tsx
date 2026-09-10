import React from "react"
import { FeedNativeAd } from "./FeedNativeAd"
import { useSponsorshipAd, useSponsorshipLoaded } from "./SponsorshipProvider"

interface AdSlotProps {
  slot: "feed_native" | "sidebar_top" | "post_below_title" | "pages_header"
  className?: string
}

/**
 * Renders a house ad for `slot` from SponsorshipProvider (no per-slot fetch).
 * Must be under a SponsorshipProvider that requested this slot.
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
        <img src={campaign.creativeImageUrl!} alt={campaign.name} className="max-w-full h-auto rounded" />
      )}
      {hasHtml && (
        <div className="sponsorship-html" dangerouslySetInnerHTML={{ __html: campaign.creativeHtml! }} />
      )}
      {!hasImage && !hasHtml && <span className="text-sm text-muted">{campaign.name}</span>}
      <div className="text-[10px] uppercase tracking-wide text-muted mt-1">Sponsored</div>
    </a>
  )
}
