import { http, Result } from "@fider/services/http"
import { PublicSponsorshipCampaign, SponsorshipCampaign, SponsorshipPackage } from "@fider/models"

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

/** One request for many slots. Missing/empty slots are null. */
export const getActiveSponsorshipMap = (
  slots: string[],
  locale: string
): Promise<Result<Record<string, PublicSponsorshipCampaign | null>>> => {
  const q = new URLSearchParams({ slots: slots.join(","), locale })
  return http.get(`/api/v1/sponsorship/active?${q.toString()}`)
}
