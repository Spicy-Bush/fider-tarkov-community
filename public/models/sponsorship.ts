export interface SponsorshipPackage {
  id: number
  slug: string
  name: string
  description: string
  slots: string
  durationDays: number
  sort: number
  createdAt?: string
}

export interface SponsorshipCampaign {
  id: number
  /** Internal billing/reference name (not shown as public disclosure). */
  name: string
  /** Public advertiser / company name shown next to Sponsored. */
  advertiser: string
  /** ISO-8601 UTC from API */
  startAt: string
  endAt: string
  weight: number
  locale: string
  enabled: boolean
  clicks: number
  packageId?: number
  /** OCC token; required on update. */
  configVersion: number
  createdAt?: string
  updatedAt?: string
}

/** Safe public payload from POST /api/v1/ads/select. */
export interface PublicAd {
  campaignId: number
  advertiser: string
  placementId: string
  creativeVersionId: number
  imageUrl: string
  html: string
  clickPath: string
}

export interface AdPlacement {
  id: string
  name: string
  description: string
  kind: string
  maxWidth?: number
  maxHeight?: number
  sort: number
  enabled: boolean
}

export interface CreativeVersion {
  id: number
  campaignId: number
  versionNo: number
  imageUrl: string
  html: string
  clickUrl: string
  createdAt?: string
}

export interface CampaignAssignment {
  id: number
  campaignId: number
  placementId: string
  creativeVersionId: number
}

export const SPONSORSHIP_SLOTS = ["feed_native", "sidebar_top", "post_below_title", "pages_header"] as const
export type SponsorshipSlot = (typeof SPONSORSHIP_SLOTS)[number]

/** Display + creative guidance per placement (UI sizing). IDs come from ad_placements. */
export const SPONSORSHIP_SLOT_SPECS: Record<
  string,
  { label: string; recommended: string; imgClassName: string; frameClassName: string }
> = {
  feed_native: {
    label: "Feed (native)",
    recommended: "1200\u00d7675 (16:9), JPG/WebP",
    imgClassName: "w-full h-full object-cover",
    frameClassName: "w-full aspect-video overflow-hidden rounded bg-surface-alt",
  },
  sidebar_top: {
    label: "Sidebar",
    recommended: "600\u00d7500 (approx 6:5), JPG/WebP",
    imgClassName: "w-full h-auto block",
    frameClassName: "w-full overflow-hidden rounded bg-surface-alt",
  },
  post_below_title: {
    label: "Below post title",
    recommended: "1200\u00d7400 (wide banner), JPG/WebP",
    imgClassName: "w-full h-full object-cover",
    frameClassName: "w-full aspect-[3/1] overflow-hidden rounded bg-surface-alt",
  },
  pages_header: {
    label: "Pages header",
    recommended: "1200\u00d7280 (wide banner), JPG/WebP",
    imgClassName: "w-full h-full object-cover",
    frameClassName: "w-full aspect-[4/1] overflow-hidden rounded bg-surface-alt",
  },
}

export const FEED_AD_EVERY = 5

export type CampaignDerivedStatus = "disabled" | "scheduled" | "active" | "ended"

export function deriveCampaignStatus(c: { enabled: boolean; startAt: string; endAt: string }, now = new Date()): CampaignDerivedStatus {
  if (!c.enabled) return "disabled"
  const start = new Date(c.startAt).getTime()
  const end = new Date(c.endAt).getTime()
  const t = now.getTime()
  if (Number.isNaN(start) || Number.isNaN(end)) return "disabled"
  if (t < start) return "scheduled"
  if (t >= end) return "ended"
  return "active"
}
