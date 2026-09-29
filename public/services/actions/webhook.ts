import { http, Result, StringObject } from "@fider/services"
import { WebhookData, WebhookPreviewResult, WebhookTriggerResult, WebhookType } from "@fider/models"

export const createWebhook = async (data: WebhookData): Promise<Result<{ id: number }>> => {
  return await http.post(`/api/admin/webhook`, data)
}

export const updateWebhook = async (id: number, data: WebhookData): Promise<Result> => {
  return await http.put(`/api/admin/webhook/${id}`, data)
}

export const deleteWebhook = async (id: number): Promise<Result> => {
  return await http.delete(`/api/admin/webhook/${id}`)
}

export const testWebhook = async (id: number): Promise<Result<WebhookTriggerResult>> => {
  return await http.post(`/api/admin/webhook/test/${id}`)
}

export const previewWebhook = async (type: WebhookType, url: string, content: string): Promise<Result<WebhookPreviewResult>> => {
  return await http.post("/api/admin/webhook/preview", { type, url, content })
}

export const getWebhookHelp = async (type: WebhookType): Promise<Result<StringObject>> => {
  return await http.get(`/api/admin/webhook/props/${type}`)
}
