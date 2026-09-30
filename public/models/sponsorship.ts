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
