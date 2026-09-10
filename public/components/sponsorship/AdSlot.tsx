import React from "react"
import { FeedNativeAd } from "./FeedNativeAd"
import { HtmlCreativeFrame } from "./HtmlCreativeFrame"
import { AdSenseSlot } from "./AdSenseSlot"
import { getAdSenseClient } from "./adsenseClient"
import { resolvePlacementAdConfig, usePlacementAdConfig } from "./usePlacementAdConfig"
import { PublicAd, SPONSORSHIP_SLOT_SPECS } from "@fider/models"

export interface AdSlotProps {
  instanceId: string
  placementId: string
  /** undefined = parent still loading / select failed; null = no house fill; PublicAd = render house */
  ad: PublicAd | null | undefined
  className?: string
  /**
   * When true (default on public pages), null house fill may show AdSense if client+slot configured.
   * Admin preview must pass false — house creatives only, never live AdSense.
   */
  allowAdSense?: boolean
  /** When select HTTP failed — never swap AdSense for that load. */
  selectFailed?: boolean
}

/**
 * Props-only renderer. House fill from select takes precedence (never also AdSense).
 * null + client + placement adsense slot → AdSenseSlot; else emptyPolicy collapse|reserve.
 * HTML via sandboxed iframe only. No dangerouslySetInnerHTML.
 */
export const AdSlot: React.FC<AdSlotProps> = ({
  instanceId,
  placementId,
  ad,
  className,
  allowAdSense = true,
  selectFailed = false,
}) => {
  const wantConfig = allowAdSense && !selectFailed && ad === null
  const { config: placementConfig, loaded: configLoaded } = usePlacementAdConfig(wantConfig)

  // Loading select or failed select: render nothing (do not treat as empty inventory).
  if (ad === undefined || selectFailed) {
    return null
  }

  // House fill — never also AdSense for this instance.
  if (ad !== null) {
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
        data-ad-network="house"
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

  // ad === null: empty house fill → optional AdSense / emptyPolicy
  if (!allowAdSense) {
    return null
  }

  if (!configLoaded) {
    return null
  }

  const cfg = resolvePlacementAdConfig(placementId, placementConfig)
  const client = getAdSenseClient()
  const slotId = (cfg.adsenseSlotId || "").trim()

  if (client && slotId) {
    return (
      <AdSenseSlot
        client={client}
        slotId={slotId}
        format={cfg.adsenseFormat || "auto"}
        placementId={placementId}
        instanceId={instanceId}
        className={className}
      />
    )
  }

  if (cfg.emptyPolicy === "reserve") {
    const spec = SPONSORSHIP_SLOT_SPECS[placementId] || SPONSORSHIP_SLOT_SPECS.sidebar_top
    return (
      <div
        className={className || "block my-3"}
        data-ad-instance={instanceId}
        data-ad-placement={placementId}
        data-ad-empty="reserve"
        aria-hidden
      >
        <div className={spec.frameClassName} />
      </div>
    )
  }

  return null
}
