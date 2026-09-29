import { Failure, http, RequestError } from "@fider/services/http"
import { retryRequest } from "../retryRequest"
import { ImageUpload } from "@fider/models"

export type FileType = "all" | "files" | "attachments" | "avatars" | "logos" | "pages" | "oauth"
export type FileUsage = "all" | "used" | "unused"

export interface FileInfo {
  name: string
  blobKey: string
  size: number
  contentType: string
  createdAt: string
  isInUse: boolean
  hasProtectedReferences: boolean
  width: number
  height: number
  thumbnailURL: string
  url: string
  state: "ready" | "deleting"
  lastError?: string
}

export interface FileQuery {
  page: number
  pageSize: number
  search: string
  type: FileType
  usage: FileUsage
  includeDeleted: boolean
  includeDrafts: boolean
  sortBy: "name" | "size" | "createdAt"
  sortDir: "asc" | "desc"
}

export interface FileLibraryOptions {
  defaults: FileQuery
  types: Array<{ value: FileType; label: string }>
  usage: Array<{ value: FileUsage; label: string }>
  sort: Array<{ value: FileQuery["sortBy"]; label: string }>
  pageSizes: number[]
  maxPage: number
  maxPageSize: number
  maxImageBytes: number
}

export interface FileListResponse {
  files: FileInfo[]
  total: number
  totalBytes?: number
  page: number
  pageSize: number
  totalPages: number
  listedAt: string
  inventory: { state: "pending" | "retrying" | "ready"; scanned: number; skipped: number; lastError?: string }
}

export interface FileUploadRequest {
  submissionId: string
  name: string
  file: ImageUpload
  uploadType: "file" | "attachment"
}

export interface FileUsageResponse {
  items: Array<{ kind: string; id: number; title: string; url: string; scope: "active" | "deleted" | "draft" }>
  total: number
  page: number
  totalPages: number
}

export interface DeleteFilesResponse {
  deleted: string[]
  pending: string[]
  skipped: string[]
  errors: Array<{ blobKey: string; message: string }>
}

export interface PruneFilesRequest {
  search: string
  type: FileType
  before: string
  includeDeleted: boolean
  includeDrafts: boolean
  cursor?: string
}

export interface PruneFilesResponse extends DeleteFilesResponse {
  nextCursor?: string
}

export const listFiles = (query: FileQuery, signal: AbortSignal) => {
  const params = new URLSearchParams({
    ...query,
    page: String(query.page),
    pageSize: String(query.pageSize),
    includeDeleted: String(query.includeDeleted),
    includeDrafts: String(query.includeDrafts),
  })
  return http.get<FileListResponse>(`/api/admin/files?${params}`, { signal, notifyOnError: false })
}

export const uploadFile = (request: FileUploadRequest) => {
  return retryRequest(() => http.post<FileInfo>("/api/admin/files", request, { notifyOnError: false }), { delayMs: 250 })
}

export const renameFile = (blobKey: string, name: string) => {
  return http.put<FileInfo>("/api/admin/files/name", { blobKey, name }, { notifyOnError: false })
}

export const refreshFileInventory = () => {
  return http.post("/api/admin/files/refresh", {}, { notifyOnError: false })
}

export interface FileRemoval {
  force: boolean
  includeDeleted: boolean
  includeDrafts: boolean
}

export const deleteFiles = (blobKeys: string[], removal: FileRemoval) => {
  return retryRequest(() => http.post<DeleteFilesResponse>("/api/admin/files/delete", { blobKeys, ...removal }, { notifyOnError: false }), { delayMs: 250 })
}

export const getFileUsage = (blobKey: string, page: number, signal: AbortSignal) => {
  const params = new URLSearchParams({ key: blobKey, page: String(page) })
  return http.get<FileUsageResponse>(`/api/admin/files/usage?${params}`, { signal, notifyOnError: false })
}

export const pruneFiles = (request: PruneFilesRequest) => {
  return retryRequest(() => http.post<PruneFilesResponse>("/api/admin/files/prune", request, { notifyOnError: false }), { delayMs: 250 })
}

export const fileDownloadURL = (blobKey: string, inline = false) => {
  const params = new URLSearchParams({ key: blobKey })
  if (inline) params.set("inline", "true")
  return `/api/admin/files/download?${params}`
}

export const filePreviewURL = (blobKey: string) => {
  const params = new URLSearchParams({ key: blobKey, size: "512" })
  return `/api/admin/files/thumbnail?${params}`
}

export const fileRequestFailure = (cause: unknown, message: string): Failure => {
  if (!(cause instanceof RequestError)) throw cause
  return { errors: [{ message }] }
}
