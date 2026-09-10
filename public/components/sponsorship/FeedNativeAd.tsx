import React from "react"
import { PlacementRenderMeta, PublicAd, resolvePlacementRenderMeta } from "@fider/models"
import { HStack, VStack } from "@fider/components/layout"
import { HtmlCreativeFrame } from "./HtmlCreativeFrame"

interface FeedNativeAdProps {
  ad: PublicAd
  className?: string
  /** Optional catalog meta for sizing/label; falls back to feed_native specs. */
  placement?: PlacementRenderMeta
}

/** Post-like native feed creative. Props only; empty advertiser -> null (#39). */
export const FeedNativeAd: React.FC<FeedNativeAdProps> = ({ ad, className, placement }) => {
  const advertiser = (ad.advertiser || "").trim()
  if (!advertiser) {
    return null
  }

  const meta = resolvePlacementRenderMeta(ad.placementId || "feed_native", placement)
  const hasImage = Boolean(ad.imageUrl)
  const hasHtml = Boolean(ad.html)
  const disclosure = `Sponsored - ${advertiser}`
  const rootClassName = ["block", "no-underline", "text-inherit", className].filter(Boolean).join(" ")
  const sizeStyle =
    meta.maxWidth || meta.maxHeight
      ? {
          ...(meta.maxWidth ? { maxWidth: meta.maxWidth } : {}),
          ...(meta.maxHeight ? { maxHeight: meta.maxHeight } : {}),
        }
      : undefined

  return (
    <a
      href={ad.clickPath}
      className={rootClassName}
      rel="sponsored noopener"
      target="_blank"
      aria-label={disclosure}
      data-ad-network="house"
      data-ad-kind="native"
    >
      <HStack spacing={4} align="start" className="min-w-0 opacity-95">
        <div className="shrink-0 w-10 text-center text-[10px] uppercase text-muted leading-tight pt-1">Ad</div>
        <VStack className="flex-1 min-w-0" spacing={2}>
          <div className="text-[10px] uppercase tracking-wide text-muted">{disclosure}</div>
          <div className="text-lg font-medium text-primary wrap-anywhere">{advertiser}</div>
          {hasImage && (
            <div className={meta.frameClassName} style={sizeStyle}>
              <img src={ad.imageUrl} alt="" className={meta.imgClassName} loading="lazy" />
            </div>
          )}
          {hasHtml && (
            <div onClick={(e) => e.preventDefault()} style={sizeStyle}>
              <HtmlCreativeFrame html={ad.html} title={disclosure} className="sponsorship-html-frame w-full border-0 rounded bg-surface-alt min-h-[60px] text-muted text-sm" />
            </div>
          )}
        </VStack>
      </HStack>
    </a>
  )
}
