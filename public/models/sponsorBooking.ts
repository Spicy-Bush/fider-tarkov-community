export interface SponsorBooking {
  placementId: string
  share: number
}

export interface SponsorCampaign {
  id: number
  revision: number
  name: string
  advertiser: string
  category: string
  exclusive: boolean
  state: "draft" | "reserved" | "booked" | "paused" | "cancelled"
  startAt: string
  endAt: string
  confirmBy: string | null
  languages: string[]
  countries: string[]
  pageTypes: string[]
  bookings: SponsorBooking[]
  amountMinor: number
  currency: string
  paymentStatus: "unpaid" | "partial" | "paid"
  notes: string
}

export interface SponsorArtwork {
  framed: boolean
  headline: string
  description: string
  imageKey: string
  bannerCrop?: SponsorImageCrop
  logoKey: string
  callToAction: string
  destination: string
  offerCode: string
  offerTerms: string
  offerExpires: string | null
}

export interface SponsorImageCrop {
  x: number
  y: number
  zoom: number
}

export interface SponsorCreative extends SponsorArtwork {
  id: number
  revision: number
  campaignId: number
  state: "draft" | "review" | "approved" | "rejected"
  reviewReason: string
  language: string
  device: string
  startAt: string | null
  endAt: string | null
}

export interface SponsorCampaignSave {
  campaign: SponsorCampaign
  creative: SponsorCreative | null
}

export interface SponsorConflicts {
  campaign: (keyof Omit<SponsorCampaign, "id" | "revision" | "bookings">)[] | null
  creative: (keyof Omit<SponsorCreative, "id" | "revision" | "campaignId" | "imageKey" | "bannerCrop">)[] | null
  bookings: string[] | null
  image: boolean
}

export type SponsorSaveConflict = {
  kind: "conflict"
  fields: SponsorConflicts
  problems: string[]
  saved: SponsorCampaignSave
  draft: SponsorCampaignSave
}

export type SponsorSaveResult = { kind: "saved"; saved: SponsorCampaignSave } | SponsorSaveConflict

export interface SponsorCampaignEdit extends SponsorCampaignSave {
  baseCampaign?: SponsorCampaign
  baseCreative?: SponsorCreative | null
}

export interface SponsorPlacement {
  id: string
  name: string
  pageType: string
  device: string
  enabled: boolean
  position: string
  every: number
  empty: "none" | "kofi" | "adsense"
  adsenseSlotId?: string
}

export interface SponsorContext {
  pageType: string
  id: number
  language: string
  device: string
}

export interface SponsorOpportunity {
  instanceId: string
  placementId: string
}

export type SponsorSelection =
  | { kind: "none"; placement: SponsorPlacement }
  | { kind: "kofi"; placement: SponsorPlacement }
  | { kind: "adsense"; placement: SponsorPlacement }
  | { kind: "sponsor"; placement: SponsorPlacement; advertiser: string; creative: SponsorArtwork; expiresAt: string; clickUrl: string }

export interface SponsorExclusion {
  pageType: "post" | "page"
  id: number
  reason: string
}

export interface SponsorManagement {
  browse: { page: number; campaignId: number; artworkPage: number; search: string }
  nextPage: boolean
  nextArtwork: boolean
  campaigns: SponsorCampaign[]
  creatives: SponsorCreative[]
  placements: SponsorPlacement[]
  exclusions: SponsorExclusion[]
}

export interface SponsorReport {
  allocations: {
    day: string
    campaignId: number
    placementId: string
    creativeId: number
    eligible: number
    clicks: number
    allocated: number
    expectedHundredths: number
  }[]
  changes: { campaignId: number; state: string; at: string }[]
}
