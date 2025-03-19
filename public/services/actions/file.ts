import { http, Result } from "@fider/services/http"
import { ImageUpload } from "@fider/models"

export interface FileInfo {
  name: string
  blobKey: string
  size: number
  contentType: string
  createdAt: string
  isInUse: boolean
  usedIn?: string[]
}

export interface FileListResponse {
  files: FileInfo[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

export interface FileUploadRequest {
  name: string
  file: ImageUpload
  uploadType: "file" | "attachment"
}

export const listFiles = async (url: string): Promise<Result<FileListResponse>> => {
  return await http.get<FileListResponse>(url)
}

export const uploadFile = async (request: FileUploadRequest): Promise<Result<FileInfo>> => {
  return await http.post<FileInfo>("/api/v1/admin/files", request)
}

export const renameFile = async (blobKey: string, newName: string): Promise<Result<FileInfo>> => {
  return await http.put<FileInfo>(`/api/v1/admin/files/${blobKey}`, { name: newName })
}

export const deleteFile = async (blobKey: string, force: boolean = false): Promise<Result<void>> => {
  const url = force 
    ? `/api/v1/admin/files/${blobKey}?force=true` 
    : `/api/v1/admin/files/${blobKey}`;
    
  return await http.delete(url)
}

export const getFileUsage = async (blobKey: string): Promise<Result<string[]>> => {
  return await http.get<string[]>(`/api/v1/admin/files/${blobKey}/usage`)
}