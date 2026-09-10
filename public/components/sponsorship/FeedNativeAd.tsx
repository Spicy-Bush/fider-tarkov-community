import React from "react"
import { PublicSponsorshipCampaign } from "@fider/models"
import { HStack, VStack } from "@fider/components/layout"

interface FeedNativeAdProps {
  campaign: PublicSponsorshipCampaign
  className?: string
}

/** Post-like native feed creative (Zaddish). */
export const FeedNativeAd: React.FC<FeedNativeAdProps> = ({ campaign, className }) => {
  return (
    <a
      href={campaign.clickPath}
      className={`block no-underline text-inherit ${className || ""}`}
      rel="sponsored noopener"
      target="_blank"
    >
      <HStack spacing={4} align="start" className="min-w-0 opacity-95">
        <div className="shrink-0 w-10 text-center text-[10px] uppercase text-muted leading-tight pt-1">Ad</div>
        <VStack className="flex-1 min-w-0" spacing={2}>
          <div className="text-[10px] uppercase tracking-wide text-muted">Sponsored</div>
          <div className="text-lg font-medium text-primary wrap-anywhere">{campaign.name}</div>
          {campaign.creativeHtml ? (
            <div className="text-muted text-sm sponsorship-html" dangerouslySetInnerHTML={{ __html: campaign.creativeHtml }} />
          ) : campaign.creativeImageUrl ? (
            <img src={campaign.creativeImageUrl} alt="" className="max-w-full h-auto rounded" />
          ) : null}
        </VStack>
      </HStack>
    </a>
  )
}
