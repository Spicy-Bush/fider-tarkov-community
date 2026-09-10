import { http, Result } from "@fider/services/http"
import {
  AdPlacement,
  PlacementAdConfig,
  CampaignAssignment,
  CreativeVersion,
  PublicAd,
  SponsorshipCampaign,
  SponsorshipPackage,
} from "@fider/models"

export const listSponsorshipPackages = (): Promise<Result<SponsorshipPackage[]>> => {
  return http.get<SponsorshipPackage[]>("/api/v1/sponsorship/packages")
}

export const createSponsorshipPackage = (body: {
  slug: string
  name: string
  description: string
  slots: string
  durationDays: number
  sort: number
}): Promise<Result<SponsorshipPackage>> => {
  return http.post<SponsorshipPackage>("/api/v1/sponsorship/packages", body)
}

export const updateSponsorshipPackage = (
  id: number,
  body: {
    slug: string
    name: string
    description: string
    slots: string
    durationDays: number
    sort: number
  }
): Promise<Result<SponsorshipPackage>> => {
  return http.put<SponsorshipPackage>(`/api/v1/sponsorship/packages/${id}`, body)
}

export const deleteSponsorshipPackage = (id: number): Promise<Result> => {
  return http.delete(`/api/v1/sponsorship/packages/${id}`)
}

export const listSponsorshipCampaigns = (): Promise<Result<SponsorshipCampaign[]>> => {
  return http.get<SponsorshipCampaign[]>("/api/v1/sponsorship/campaigns")
}

export type CreateCampaignWithGraphBody = Record<string, unknown> & {
  version?: { imageUrl: string; html: string; clickUrl: string }
  assignments?: { placementId: string; creativeVersionId?: number }[]
}

export type CreateCampaignGraphResult = {
  campaign: SponsorshipCampaign
  versions: CreativeVersion[]
  assignments: CampaignAssignment[]
  configVersion: number
}

export const createSponsorshipCampaign = (
  body: CreateCampaignWithGraphBody
): Promise<Result<SponsorshipCampaign | CreateCampaignGraphResult>> => {
  return http.post<SponsorshipCampaign | CreateCampaignGraphResult>("/api/v1/sponsorship/campaigns", body)
}

export const updateSponsorshipCampaign = (
  id: number,
  body: Record<string, unknown>
): Promise<Result<SponsorshipCampaign>> => {
  return http.put<SponsorshipCampaign>(`/api/v1/sponsorship/campaigns/${id}`, body)
}

export const deleteSponsorshipCampaign = (id: number): Promise<Result> => {
  return http.delete(`/api/v1/sponsorship/campaigns/${id}`)
}

export type AdSelectRequestSlot = { instanceId: string; placementId: string }

/** Page-owned selection. Response keyed by instanceId; missing fills are null. */
export const selectAds = (
  slots: AdSelectRequestSlot[],
  locale: string
): Promise<Result<Record<string, PublicAd | null>>> => {
  const q = new URLSearchParams({ locale })
  return http.post(`/api/v1/ads/select?${q.toString()}`, { slots })
}

export const listAdPlacements = (): Promise<Result<AdPlacement[]>> => {
  return http.get<AdPlacement[]>("/api/v1/ads/placements")
}

export const updateAdPlacement = (
  id: string,
  body: { adsenseSlotId?: string; adsenseFormat?: string; emptyPolicy?: "collapse" | "reserve" }
): Promise<Result<AdPlacement>> => {
  return http.put<AdPlacement>(`/api/v1/ads/placements/${encodeURIComponent(id)}`, body)
}

export const listCreativeVersions = (campaignId: number): Promise<Result<CreativeVersion[]>> => {
  return http.get<CreativeVersion[]>(`/api/v1/sponsorship/campaigns/${campaignId}/versions`)
}

export type CreateCreativeVersionResult = {
  version: CreativeVersion
  configVersion: number
}

export const createCreativeVersion = (
  campaignId: number,
  body: { kind: string; imageUrl: string; html: string; clickUrl: string; configVersion: number }
): Promise<Result<CreateCreativeVersionResult>> => {
  return http.post<CreateCreativeVersionResult>(`/api/v1/sponsorship/campaigns/${campaignId}/versions`, body)
}

export const listCampaignAssignments = (campaignId: number): Promise<Result<CampaignAssignment[]>> => {
  return http.get<CampaignAssignment[]>(`/api/v1/sponsorship/campaigns/${campaignId}/assignments`)
}

export type CampaignGraphSaveBody = {
  name: string
  advertiser: string
  startAt: string
  endAt: string
  weight: number
  locale: string
  enabled: boolean
  packageId?: number
  configVersion: number
  assignments: { placementId: string; creativeVersionId: number }[]
}

export type CampaignGraphSaveResult = {
  campaign: SponsorshipCampaign
  assignments: CampaignAssignment[]
}

/** Atomic OCC save: campaign fields + full assignment set in one txn. */
export const saveSponsorshipCampaignGraph = (
  campaignId: number,
  body: CampaignGraphSaveBody
): Promise<Result<CampaignGraphSaveResult>> => {
  return http.put<CampaignGraphSaveResult>(`/api/v1/sponsorship/campaigns/${campaignId}/graph`, body)
}

/** Public placement AdSense / empty-policy catalog (enabled rows only). */
export const getAdPlacementConfig = (): Promise<Result<Record<string, PlacementAdConfig>>> => {
  return http.get<Record<string, PlacementAdConfig>>("/api/v1/ads/placement-config")
}
