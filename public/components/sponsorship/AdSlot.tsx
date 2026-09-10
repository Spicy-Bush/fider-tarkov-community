import React, { useEffect, useState } from "react"
import { PublicSponsorshipCampaign } from "@fider/models"
import { actions, Fider } from "@fider/services"
import { FeedNativeAd } from "./FeedNativeAd"

interface AdSlotProps {
  slot: "feed_native" | "sidebar_top" | "post_below_title" | "pages_header"
  className?: string
}

function siteLocale(): string {
  const loc = (Fider.currentLocale || "en").toLowerCase()
  if (loc.startsWith("ru")) return "ru"
  return "en"
}

function isCampaign(data: PublicSponsorshipCampaign | Record<string, never> | undefined): data is PublicSponsorshipCampaign {
  return !!data && typeof (data as PublicSponsorshipCampaign).id === "number" && (data as PublicSponsorshipCampaign).id > 0
}

/** House campaign first; AdSense hook reserved; otherwise empty. */
export const AdSlot: React.FC<AdSlotProps> = ({ slot, className }) => {
  const [campaign, setCampaign] = useState<PublicSponsorshipCampaign | null>(null)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      const result = await actions.getActiveSponsorship(slot, siteLocale())
      if (cancelled) return
      if (result.ok && isCampaign(result.data)) {
        setCampaign(result.data)
      } else {
        setCampaign(null)
      }
      setLoaded(true)
    })()
    return () => {
      cancelled = true
    }
  }, [slot])

  if (!loaded) {
    return null
  }

  if (campaign) {
    if (slot === "feed_native") {
      return <FeedNativeAd campaign={campaign} className={className} />
    }
    return (
      <a
        href={campaign.clickPath}
        className={className || "block my-3 no-underline"}
        rel="sponsored noopener"
        target="_blank"
        aria-label={campaign.name}
      >
        {campaign.creativeHtml ? (
          <div className="sponsorship-html" dangerouslySetInnerHTML={{ __html: campaign.creativeHtml }} />
        ) : campaign.creativeImageUrl ? (
          <img src={campaign.creativeImageUrl} alt={campaign.name} className="max-w-full h-auto rounded" />
        ) : (
          <span className="text-sm text-muted">{campaign.name}</span>
        )}
        <div className="text-[10px] uppercase tracking-wide text-muted mt-1">Sponsored</div>
      </a>
    )
  }

  // AdSense fallback: only if publisher id is configured on window (ops-set).
  const client = (window as unknown as { __adsense_client?: string }).__adsense_client
  if (client) {
    return (
      <div className={className || "my-3"} data-ad-slot={slot} data-ad-client={client}>
        <ins
          className="adsbygoogle"
          style={{ display: "block" }}
          data-ad-client={client}
          data-ad-slot={slot}
          data-ad-format="auto"
          data-full-width-responsive="true"
        />
      </div>
    )
  }

  return null
}
