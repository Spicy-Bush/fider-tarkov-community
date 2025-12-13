import { http, Result } from "@fider/services/http"

export interface CannedResponse {
  id: number
  type: string
  title: string
  content: string
  duration?: string
  isActive: boolean
  createdAt: string
  createdBy?: number
}

export const listCannedResponses = async (type: string): Promise<Result<CannedResponse[]>> => {
  return await http.get<CannedResponse[]>(`/api/v1/responses/${type}`)
}

export const createCannedResponse = async (data: {
  type: string
  title: string
  content: string
  duration?: string
}): Promise<Result<CannedResponse>> => {
  return await http.post<CannedResponse>("/api/v1/responses", data)
}

export const updateCannedResponse = async (id: number, data: {
  type: string
  title: string
  content: string
  duration?: string
  isActive: boolean
}): Promise<Result<CannedResponse>> => {
  return await http.put<CannedResponse>(`/api/v1/responses/${id}`, data)
}

export const deleteCannedResponse = async (id: number): Promise<Result<void>> => {
  return await http.delete(`/api/v1/responses/${id}`)
} 