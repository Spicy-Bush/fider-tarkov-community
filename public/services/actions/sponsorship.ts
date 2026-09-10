import { http, Result } from "@fider/services/http"
import {
  AdPlacement,
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

export const createSponsorshipCampaign = (body: Record<string, unknown>): Promise<Result<SponsorshipCampaign>> => {
  return http.post<SponsorshipCampaign>("/api/v1/sponsorship/campaigns", body)
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

export const listCreativeVersions = (campaignId: number): Promise<Result<CreativeVersion[]>> => {
  return http.get<CreativeVersion[]>(`/api/v1/sponsorship/campaigns/${campaignId}/versions`)
}

export const createCreativeVersion = (
  campaignId: number,
  body: { imageUrl: string; html: string; clickUrl: string; configVersion: number }
): Promise<Result<CreativeVersion>> => {
  return http.post<CreativeVersion>(`/api/v1/sponsorship/campaigns/${campaignId}/versions`, body)
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
