import { http, Result, querystring } from "@fider/services"
import { Report, ReportReason, ReportType, ReportStatus, Post, Comment } from "@fider/models"

interface CreateReportResponse {
  id: number
}

export const reportPost = async (
  postNumber: number,
  reason: string,
  details?: string
): Promise<Result<CreateReportResponse>> => {
  return http.post<CreateReportResponse>(`/api/v1/posts/${postNumber}/report`, {
    reason,
    details,
  })
}

export const reportComment = async (
  postNumber: number,
  commentId: number,
  reason: string,
  details?: string
): Promise<Result<CreateReportResponse>> => {
  return http.post<CreateReportResponse>(`/api/v1/posts/${postNumber}/comments/${commentId}/report`, {
    reason,
    details,
  })
}

export interface ListReportsParams {
  page?: number
  perPage?: number
  status?: ReportStatus | ReportStatus[]
  type?: ReportType
  reason?: string
}

export interface ReportViewer {
  userId: number
  userName: string
}

export interface ReportViewers {
  reportId: number
  viewers: ReportViewer[]
}

export interface ListReportsResponse {
  reports: Report[]
  total: number
  page: number
  perPage: number
  viewers: ReportViewers[]
}

export const listReports = async (params: ListReportsParams): Promise<Result<ListReportsResponse>> => {
  const queryParams: Record<string, string | number | undefined> = {
    page: params.page,
    perPage: params.perPage,
    type: params.type,
    reason: params.reason,
  }
  if (Array.isArray(params.status)) {
    queryParams.status = params.status.join(",")
  } else if (params.status) {
    queryParams.status = params.status
  }
  const qs = querystring.stringify(queryParams)
  return http.get<ListReportsResponse>(`/api/v1/reports${qs}`)
}

export const getReport = async (reportId: number): Promise<Result<Report>> => {
  return http.get<Report>(`/api/v1/reports/${reportId}`)
}

export interface ReportDetailsResponse {
  report: Report
  post?: Post
  comment?: Comment
}

export const getReportDetails = async (reportId: number): Promise<Result<ReportDetailsResponse>> => {
  return http.get<ReportDetailsResponse>(`/api/v1/reports/${reportId}/details`)
}

export const assignReport = async (reportId: number): Promise<Result> => {
  return http.post(`/api/v1/reports/${reportId}/assign`)
}

export const unassignReport = async (reportId: number): Promise<Result> => {
  return http.delete(`/api/v1/reports/${reportId}/assign`)
}

export const resolveReport = async (
  reportId: number,
  status: "resolved" | "dismissed",
  resolutionNote?: string
): Promise<Result> => {
  return http.put(`/api/v1/reports/${reportId}/resolve`, {
    status,
    resolutionNote,
  })
}

export const getReportReasons = async (): Promise<Result<ReportReason[]>> => {
  return http.get<ReportReason[]>("/api/v1/report-reasons")
}

export const reportHeartbeat = async (reportId: number): Promise<Result> => {
  return http.post(`/api/v1/reports/${reportId}/heartbeat`)
}

export const stopViewingReport = async (): Promise<Result> => {
  return http.delete("/api/mod/viewing")
}

export const listAllReportReasons = async (): Promise<Result<ReportReason[]>> => {
  return http.get<ReportReason[]>("/api/v1/report-reasons/all")
}

export const createReportReason = async (data: {
  title: string
  description?: string
}): Promise<Result<{ id: number }>> => {
  return http.post<{ id: number }>("/api/v1/report-reasons", data)
}

export const updateReportReason = async (
  id: number,
  data: {
    title: string
    description?: string
    isActive: boolean
  }
): Promise<Result> => {
  return http.put(`/api/v1/report-reasons/${id}`, data)
}

export const deleteReportReason = async (id: number): Promise<Result> => {
  return http.delete(`/api/v1/report-reasons/${id}`)
}

export const reorderReportReasons = async (ids: number[]): Promise<Result> => {
  return http.put("/api/v1/admin/report-reasons-order", { ids })
}
