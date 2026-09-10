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
  name: string
  /** Comma-separated slot ids — one campaign may cover many placements. */
  slots: string
  creativeImageUrl: string
  creativeHtml: string
  clickUrl: string
  /** ISO-8601 UTC from API */
  startAt: string
  endAt: string
  weight: number
  locale: string
  enabled: boolean
  clicks: number
  packageId?: number
  createdAt?: string
  updatedAt?: string
}

export interface PublicSponsorshipCampaign {
  id: number
  name: string
  slotId: string
  creativeImageUrl?: string
  creativeHtml?: string
  clickPath: string
}

export const SPONSORSHIP_SLOTS = ["feed_native", "sidebar_top", "post_below_title", "pages_header"] as const
export type SponsorshipSlot = (typeof SPONSORSHIP_SLOTS)[number]
