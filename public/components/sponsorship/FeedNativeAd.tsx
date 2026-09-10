import React from "react"
import { PublicSponsorshipCampaign, SPONSORSHIP_SLOT_SPECS } from "@fider/models"
import { HStack, VStack } from "@fider/components/layout"

interface FeedNativeAdProps {
  campaign: PublicSponsorshipCampaign
  className?: string
}

/** Native feed creative. Requires advertiser; returns null if missing. */
export const FeedNativeAd: React.FC<FeedNativeAdProps> = ({ campaign, className }) => {
  const advertiser = (campaign.advertiser || "").trim()
  if (!advertiser) {
    return null
  }

  const spec = SPONSORSHIP_SLOT_SPECS.feed_native
  const hasImage = Boolean(campaign.creativeImageUrl)
  const hasHtml = Boolean(campaign.creativeHtml)
  const disclosure = `Sponsored · ${advertiser}`
  const rootClassName = ["block", "no-underline", "text-inherit", className].filter(Boolean).join(" ")

  return (
    <a
      href={campaign.clickPath}
      className={rootClassName}
      rel="sponsored noopener"
      target="_blank"
      aria-label={disclosure}
    >
      <HStack spacing={4} align="start" className="min-w-0 opacity-95">
        <div className="shrink-0 w-10 text-center text-[10px] uppercase text-muted leading-tight pt-1">Ad</div>
        <VStack className="flex-1 min-w-0" spacing={2}>
          <div className="text-[10px] uppercase tracking-wide text-muted">{disclosure}</div>
          <div className="text-lg font-medium text-primary wrap-anywhere">{advertiser}</div>
          {hasImage && (
            <div className={spec.frameClassName}>
              <img
                src={campaign.creativeImageUrl!}
                alt=""
                className={spec.imgClassName}
                loading="lazy"
              />
            </div>
          )}
          {hasHtml && (
            <div className="text-muted text-sm sponsorship-html" dangerouslySetInnerHTML={{ __html: campaign.creativeHtml! }} />
          )}
        </VStack>
      </HStack>
    </a>
  )
}
