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
  return http.get<SponsorshipPackage[]>("/api/sponsorship/packages")
}

export const createSponsorshipPackage = (body: {
  slug: string
  name: string
  description: string
  slots: string
  durationDays: number
  sort: number
}): Promise<Result<SponsorshipPackage>> => {
  return http.post<SponsorshipPackage>("/api/sponsorship/packages", body)
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
  return http.put<SponsorshipPackage>(`/api/sponsorship/packages/${id}`, body)
}

export const deleteSponsorshipPackage = (id: number): Promise<Result> => {
  return http.delete(`/api/sponsorship/packages/${id}`)
}

export const listSponsorshipCampaigns = (): Promise<Result<SponsorshipCampaign[]>> => {
  return http.get<SponsorshipCampaign[]>("/api/sponsorship/campaigns")
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
  return http.post<SponsorshipCampaign | CreateCampaignGraphResult>("/api/sponsorship/campaigns", body)
}

export const updateSponsorshipCampaign = (
  id: number,
  body: Record<string, unknown>
): Promise<Result<SponsorshipCampaign>> => {
  return http.put<SponsorshipCampaign>(`/api/sponsorship/campaigns/${id}`, body)
}

export const deleteSponsorshipCampaign = (id: number): Promise<Result> => {
  return http.delete(`/api/sponsorship/campaigns/${id}`)
}

export type AdSelectRequestSlot = { instanceId: string; placementId: string }

/** Page-owned selection. Response keyed by instanceId; missing fills are null. */
export const selectAds = (
  slots: AdSelectRequestSlot[],
  locale: string,
  signal?: AbortSignal
): Promise<Result<Record<string, PublicAd | null>>> => {
  const q = new URLSearchParams({ locale })
  return http.post(`/api/ads/select?${q.toString()}`, { slots }, { signal, notifyOnError: false })
}

export const listAdPlacements = (): Promise<Result<AdPlacement[]>> => {
  return http.get<AdPlacement[]>("/api/ads/placements")
}

export const updateAdPlacement = (
  id: string,
  body: { adsenseSlotId?: string; adsenseFormat?: string; emptyPolicy?: "collapse" | "reserve" }
): Promise<Result<AdPlacement>> => {
  return http.put<AdPlacement>(`/api/ads/placements/${encodeURIComponent(id)}`, body)
}

export const listCreativeVersions = (campaignId: number): Promise<Result<CreativeVersion[]>> => {
  return http.get<CreativeVersion[]>(`/api/sponsorship/campaigns/${campaignId}/versions`)
}

export type CreateCreativeVersionResult = {
  version: CreativeVersion
  configVersion: number
}

export const createCreativeVersion = (
  campaignId: number,
  body: { imageUrl: string; html: string; clickUrl: string; configVersion: number }
): Promise<Result<CreateCreativeVersionResult>> => {
  return http.post<CreateCreativeVersionResult>(`/api/sponsorship/campaigns/${campaignId}/versions`, body)
}

export const listCampaignAssignments = (campaignId: number): Promise<Result<CampaignAssignment[]>> => {
  return http.get<CampaignAssignment[]>(`/api/sponsorship/campaigns/${campaignId}/assignments`)
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
  return http.put<CampaignGraphSaveResult>(`/api/sponsorship/campaigns/${campaignId}/graph`, body)
}

/** Public placement AdSense / empty-policy catalog (enabled rows only). */
export const getAdPlacementConfig = (): Promise<Result<Record<string, PlacementAdConfig>>> => {
  return http.get<Record<string, PlacementAdConfig>>("/api/ads/placement-config")
}
