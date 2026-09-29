import { RetryResult, retryRequest } from "../retryRequest"
import { http, Result } from "@fider/services/http"
import { FeedbackExportPreset, FeedbackExportRecipe, FeedbackExportRow } from "@fider/models"

export const previewFeedbackExport = async (
  recipe: FeedbackExportRecipe,
  seed: string,
  signal: AbortSignal
): Promise<Result<{ sections: FeedbackExportRow[][] }>> => {
  return await http.post<{ sections: FeedbackExportRow[][] }>("/api/admin/bsg-export/preview", { recipe, seed }, { signal, notifyOnError: false })
}

type PresetInput = Pick<FeedbackExportPreset, "id" | "name" | "recipe">

export interface PresetUpdateResult {
  preset: FeedbackExportPreset
  conflicts: ("name" | "recipe")[]
}

export const createFeedbackExportPreset = (preset: PresetInput): Promise<RetryResult<FeedbackExportPreset>> => {
  return retryRequest(() => http.post<FeedbackExportPreset>("/api/admin/bsg-export/presets", preset, { notifyOnError: false }), {
    attempts: 2,
    delayMs: 250,
  })
}

export const updateFeedbackExportPreset = async (
  submissionId: string,
  { id, name, recipe }: PresetInput,
  saved: Pick<FeedbackExportPreset, "name" | "recipe">
): Promise<Result<PresetUpdateResult>> => {
  return await http.put<PresetUpdateResult>(`/api/admin/bsg-export/presets/${id}`, { submissionId, name, recipe, saved }, { notifyOnError: false })
}

export const deleteFeedbackExportPreset = async (id: string): Promise<Result> => {
  return await http.delete(`/api/admin/bsg-export/presets/${id}`, undefined, { notifyOnError: false })
}
