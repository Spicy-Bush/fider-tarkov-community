import React from "react"
import { FeedNativeAd } from "./FeedNativeAd"
import { HtmlCreativeFrame } from "./HtmlCreativeFrame"
import { AdSenseSlot } from "./AdSenseSlot"
import { getAdSenseClient } from "./adsenseClient"
import { resolvePlacementAdConfig, usePlacementAdConfig } from "./usePlacementAdConfig"
import { PlacementRenderMeta, PublicAd, resolvePlacementRenderMeta } from "@fider/models"

export interface AdSlotProps {
  instanceId: string
  placementId: string
  /** undefined = parent still loading / select failed; null = no house fill; PublicAd = render house */
  ad: PublicAd | null | undefined
  className?: string
  /**
   * When true (default on public pages), null house fill may show AdSense if client+slot configured.
   * Admin preview must pass false -- house creatives only, never live AdSense.
   */
  allowAdSense?: boolean
  /** When select HTTP failed -- never swap AdSense for that load. */
  selectFailed?: boolean
  /** Optional catalog meta (kind/dims/label). Falls back to SPONSORSHIP_SLOT_SPECS. */
  placement?: PlacementRenderMeta
}

function frameSizeStyle(meta: { maxWidth?: number; maxHeight?: number }): React.CSSProperties | undefined {
  if (!meta.maxWidth && !meta.maxHeight) return undefined
  return {
    ...(meta.maxWidth ? { maxWidth: meta.maxWidth } : {}),
    ...(meta.maxHeight ? { maxHeight: meta.maxHeight } : {}),
  }
}

/**
 * Props-only renderer. House fill from select takes precedence (never also AdSense).
 * null + client + placement adsense slot -> AdSenseSlot; else emptyPolicy collapse|reserve.
 * HTML via sandboxed iframe only. No dangerouslySetInnerHTML.
 * When both image and html are set, show both.
 */
export const AdSlot: React.FC<AdSlotProps> = ({
  instanceId,
  placementId,
  ad,
  className,
  allowAdSense = true,
  selectFailed = false,
  placement,
}) => {
  const wantConfig = allowAdSense && !selectFailed && ad === null
  const { config: placementConfig, loaded: configLoaded } = usePlacementAdConfig(wantConfig)

  // Loading select or failed select: render nothing (do not treat as empty inventory).
  if (ad === undefined || selectFailed) {
    return null
  }

  // House fill -- never also AdSense for this instance.
  if (ad !== null) {
    const advertiser = (ad.advertiser || "").trim()
    if (!advertiser) {
      return null // #39 belt-and-suspenders
    }

    // House path: prefer explicit placement props, else SPECS/feed_native fallback (no forced catalog fetch).
    const meta = resolvePlacementRenderMeta(placementId, placement)

    if (meta.kind === "native") {
      return <FeedNativeAd ad={ad} className={className} placement={meta} />
    }

    const hasImage = Boolean(ad.imageUrl)
    const hasHtml = Boolean(ad.html)
    const disclosure = `Sponsored - ${advertiser}`
    const sizeStyle = frameSizeStyle(meta)

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
        data-ad-kind="frame"
      >
        <div className="text-[10px] uppercase tracking-wide text-muted mb-1">{disclosure}</div>
        {hasImage && (
          <div className={meta.frameClassName} style={sizeStyle}>
            <img src={ad.imageUrl} alt={advertiser} className={meta.imgClassName} loading="lazy" />
          </div>
        )}
        {hasHtml && (
          <div className="mt-2" onClick={(e) => e.preventDefault()} style={sizeStyle}>
            <HtmlCreativeFrame html={ad.html} title={disclosure} />
          </div>
        )}
        {!hasImage && !hasHtml && <span className="text-sm text-muted">{advertiser}</span>}
      </a>
    )
  }

  // ad === null: empty house fill -> optional AdSense / emptyPolicy
  if (!allowAdSense) {
    return null
  }

  if (!configLoaded) {
    return null
  }

  const cfg = resolvePlacementAdConfig(placementId, placementConfig)
  const client = getAdSenseClient()
  const slotId = (cfg.adsenseSlotId || "").trim()
  const meta = resolvePlacementRenderMeta(placementId, {
    kind: placement?.kind || cfg.kind,
    maxWidth: placement?.maxWidth ?? cfg.maxWidth,
    maxHeight: placement?.maxHeight ?? cfg.maxHeight,
    label: placement?.label,
  })

  if (client && slotId) {
    return (
      <AdSenseSlot
        client={client}
        slotId={slotId}
        format={cfg.adsenseFormat || "auto"}
        placementId={placementId}
        instanceId={instanceId}
        className={className}
        maxWidth={meta.maxWidth}
        maxHeight={meta.maxHeight}
      />
    )
  }

  if (cfg.emptyPolicy === "reserve") {
    return (
      <div
        className={className || "block my-3"}
        data-ad-instance={instanceId}
        data-ad-placement={placementId}
        data-ad-empty="reserve"
        aria-hidden
      >
        <div className={meta.frameClassName} style={frameSizeStyle(meta)} />
      </div>
    )
  }

  return null
}
