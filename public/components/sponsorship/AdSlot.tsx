import React from "react"
import { FeedNativeAd } from "./FeedNativeAd"
import { HtmlCreativeFrame } from "./HtmlCreativeFrame"
import { PublicAd, SPONSORSHIP_SLOT_SPECS } from "@fider/models"

export interface AdSlotProps {
  instanceId: string
  placementId: string
  /** undefined = parent still loading; null = no fill; PublicAd = render */
  ad: PublicAd | null | undefined
  className?: string
}

/**
 * Props-only house ad renderer. No fetch, no SponsorshipContext.
 * Empty advertiser -> null (#39). HTML via sandboxed iframe only.
 */
export const AdSlot: React.FC<AdSlotProps> = ({ instanceId, placementId, ad, className }) => {
  if (ad === undefined || ad === null) {
    return null
  }

  const advertiser = (ad.advertiser || "").trim()
  if (!advertiser) {
    return null // #39 belt-and-suspenders
  }

  if (placementId === "feed_native") {
    return <FeedNativeAd ad={ad} className={className} />
  }

  const spec = SPONSORSHIP_SLOT_SPECS[placementId] || SPONSORSHIP_SLOT_SPECS.sidebar_top
  const hasImage = Boolean(ad.imageUrl)
  const hasHtml = Boolean(ad.html)
  const disclosure = `Sponsored - ${advertiser}`

  return (
    <a
      href={ad.clickPath}
      className={className || "block my-3 no-underline"}
      rel="sponsored noopener"
      target="_blank"
      aria-label={disclosure}
      data-ad-instance={instanceId}
      data-ad-placement={placementId}
    >
      <div className="text-[10px] uppercase tracking-wide text-muted mb-1">{disclosure}</div>
      {hasImage && (
        <div className={spec.frameClassName}>
          <img src={ad.imageUrl} alt={advertiser} className={spec.imgClassName} loading="lazy" />
        </div>
      )}
      {hasHtml && (
        <div className="mt-2" onClick={(e) => e.preventDefault()}>
          <HtmlCreativeFrame html={ad.html} title={disclosure} />
        </div>
      )}
      {!hasImage && !hasHtml && <span className="text-sm text-muted">{advertiser}</span>}
    </a>
  )
}
