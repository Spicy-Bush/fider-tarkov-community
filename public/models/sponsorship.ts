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

export type EmptyAdPolicy = "collapse" | "reserve"

export interface AdPlacement {
  id: string
  name: string
  description: string
  kind: string
  maxWidth?: number
  maxHeight?: number
  sort: number
  enabled: boolean
  /** Google AdSense slot id for empty-house fallback (catalog-owned). */
  adsenseSlotId?: string
  adsenseFormat?: string
  emptyPolicy?: EmptyAdPolicy
}

/** Public slim config from GET /api/v1/ads/placement-config. */
export interface PlacementAdConfig {
  adsenseSlotId?: string
  adsenseFormat?: string
  emptyPolicy?: EmptyAdPolicy
  maxWidth?: number
  maxHeight?: number
  /** native | frame from ad_placements.kind */
  kind?: string
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
  {
    label: string
    recommended: string
    imgClassName: string
    frameClassName: string
    /** Fallback only when placement-config has not loaded; prefer DB catalog. */
    emptyPolicy?: EmptyAdPolicy
    /** Fallback kind when catalog/props absent. Catalog ad_placements.kind is source of truth. */
    kind?: "native" | "frame"
  }
> = {
  feed_native: {
    label: "Feed (native)",
    recommended: "1200\u00d7675 (16:9), JPG/WebP",
    imgClassName: "w-full h-full object-cover",
    frameClassName: "w-full aspect-video overflow-hidden rounded bg-surface-alt",
    emptyPolicy: "collapse",
    kind: "native",
  },
  sidebar_top: {
    label: "Sidebar",
    recommended: "600\u00d7500 (approx 6:5), JPG/WebP",
    imgClassName: "w-full h-auto block",
    frameClassName: "w-full overflow-hidden rounded bg-surface-alt",
    emptyPolicy: "collapse",
    kind: "frame",
  },
  post_below_title: {
    label: "Below post title",
    recommended: "1200\u00d7400 (wide banner), JPG/WebP",
    imgClassName: "w-full h-full object-cover",
    frameClassName: "w-full aspect-[3/1] overflow-hidden rounded bg-surface-alt",
    emptyPolicy: "collapse",
    kind: "frame",
  },
  pages_header: {
    label: "Pages header",
    recommended: "1200\u00d7280 (wide banner), JPG/WebP",
    imgClassName: "w-full h-full object-cover",
    frameClassName: "w-full aspect-[4/1] overflow-hidden rounded bg-surface-alt",
    emptyPolicy: "collapse",
    kind: "frame",
  },
}

/** Optional catalog/props override for renderers (kind + dims). */
export interface PlacementRenderMeta {
  kind?: string
  maxWidth?: number
  maxHeight?: number
  label?: string
}

export interface ResolvedPlacementRenderMeta {
  kind: "native" | "frame"
  label: string
  imgClassName: string
  frameClassName: string
  maxWidth?: number
  maxHeight?: number
}

/**
 * Prefer live catalog/props; fall back to SPONSORSHIP_SLOT_SPECS for SSR safety.
 * Page/catalog kind chooses the frame -- not placement id.
 */
export function resolvePlacementRenderMeta(
  placementId: string,
  override?: PlacementRenderMeta | null
): ResolvedPlacementRenderMeta {
  const spec = SPONSORSHIP_SLOT_SPECS[placementId] || SPONSORSHIP_SLOT_SPECS.sidebar_top
  const rawKind = (override?.kind || spec.kind || "frame").toLowerCase()
  const kind: "native" | "frame" = rawKind === "native" ? "native" : "frame"
  return {
    kind,
    label: override?.label || spec.label || placementId,
    imgClassName: spec.imgClassName,
    frameClassName: spec.frameClassName,
    maxWidth: override?.maxWidth,
    maxHeight: override?.maxHeight,
  }
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
