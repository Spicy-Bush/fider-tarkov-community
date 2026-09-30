import { http, Result } from "@fider/services/http"
import { SponsorshipPackage } from "@fider/models"

export const listSponsorshipPackages = (): Promise<Result<SponsorshipPackage[]>> => {
  return http.get<SponsorshipPackage[]>("/api/sponsorship/packages")
}

export const createSponsorshipPackage = (body: {
  submissionId: string
  slug: string
  name: string
  description: string
  slots: string
  durationDays: number
  sort: number
}): Promise<Result<SponsorshipPackage>> => {
  return http.post<SponsorshipPackage>("/api/sponsorship/packages", body, { notifyOnError: false })
}

export const updateSponsorshipPackage = (
  id: number,
  body: {
    submissionId: string
    slug: string
    name: string
    description: string
    slots: string
    durationDays: number
    sort: number
  }
): Promise<Result<SponsorshipPackage>> => {
  return http.put<SponsorshipPackage>(`/api/sponsorship/packages/${id}`, body, { notifyOnError: false })
}

export const deleteSponsorshipPackage = (id: number): Promise<Result> => {
  return http.delete(`/api/sponsorship/packages/${id}`, undefined, { notifyOnError: false })
}
