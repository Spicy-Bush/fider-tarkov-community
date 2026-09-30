import { http } from "@fider/services/http"
import { ImageUpload } from "@fider/models"
import { FileInfo } from "./file"
import {
  SponsorCampaignEdit, SponsorSaveResult, SponsorContext, SponsorExclusion,
  SponsorOpportunity, SponsorPlacement, SponsorReport, SponsorSelection,
} from "@fider/models/sponsorBooking"

export const uploadSponsorImage = (file: ImageUpload, submissionId: string) =>
  http.post<FileInfo>("/api/sponsorship/images", { name: file.upload?.fileName, file, submissionId }, { notifyOnError: false })

export const saveSponsorCampaign = (value: SponsorCampaignEdit, submissionId: string) =>
  http.post<SponsorSaveResult>("/api/sponsorship/bookings", { ...value, submissionId }, { notifyOnError: false })

export const deleteSponsorCampaign = (id: number) =>
  http.delete(`/api/sponsorship/bookings/${id}`, undefined, { notifyOnError: false })

export const deleteSponsorCreative = (id: number) =>
  http.delete(`/api/sponsorship/artwork/${id}`, undefined, { notifyOnError: false })

export const saveSponsorPlacement = (placement: SponsorPlacement, submissionId: string) =>
  http.post<SponsorPlacement>("/api/sponsorship/placements", { placement, submissionId }, { notifyOnError: false })

export const allocateSponsors = (context: SponsorContext, opportunities: SponsorOpportunity[], pageToken: string) =>
  http.post<Record<string, SponsorSelection>>("/api/sponsorship/select", { context, opportunities, pageToken }, { notifyOnError: false })

export const getSponsorReport = (id: number) => http.get<SponsorReport>(`/api/sponsorship/bookings/${id}/report`, { notifyOnError: false })

export const saveSponsorExclusion = (exclusion: SponsorExclusion, excluded: boolean, submissionId: string) =>
  http.post<SponsorExclusion[]>("/api/sponsorship/exclusions", { exclusion, excluded, submissionId }, { notifyOnError: false })
