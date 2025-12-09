import { User } from "./identity"

export type ReportType = "post" | "comment"

export type ReportStatus = "pending" | "in_review" | "resolved" | "dismissed"

export interface Report {
  id: number
  reportedType: ReportType
  reportedId: number
  reason: string
  details?: string
  status: ReportStatus
  createdAt: string
  reporter: User
  assignedTo?: User
  assignedAt?: string
  resolvedAt?: string
  resolvedBy?: User
  resolutionNote?: string
  postNumber?: number
  postSlug?: string
}

export interface ReportReason {
  id: number
  slug: string
  title: string
  description?: string
  sortOrder: number
  isActive: boolean
}

export const getReportTypeLabel = (type: ReportType | string): string => {
  switch (type) {
    case "post":
      return "Post"
    case "comment":
      return "Comment"
    default:
      return type
  }
}

