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

/** Display + creative guidance per placement (UI sizing + admin hints). */
export const SPONSORSHIP_SLOT_SPECS: Record<
  SponsorshipSlot,
  { label: string; recommended: string; imgClassName: string; frameClassName: string }
> = {
  feed_native: {
    label: "Feed (native)",
    recommended: "1200\u00d7675 (16:9), JPG/WebP",
    imgClassName: "w-full h-full object-cover",
    frameClassName: "w-full aspect-video max-h-52 overflow-hidden rounded bg-surface-alt",
  },
  sidebar_top: {
    label: "Sidebar",
    recommended: "600\u00d7500 (approx 6:5), JPG/WebP",
    imgClassName: "w-full h-full object-cover",
    frameClassName: "w-full aspect-[6/5] max-h-56 overflow-hidden rounded bg-surface-alt",
  },
  post_below_title: {
    label: "Below post title",
    recommended: "1200\u00d7400 (wide banner), JPG/WebP",
    imgClassName: "w-full h-full object-cover",
    frameClassName: "w-full aspect-[3/1] max-h-44 overflow-hidden rounded bg-surface-alt",
  },
  pages_header: {
    label: "Pages header",
    recommended: "1200\u00d7280 (wide banner), JPG/WebP",
    imgClassName: "w-full h-full object-cover",
    frameClassName: "w-full aspect-[4/1] max-h-36 overflow-hidden rounded bg-surface-alt",
  },
}

