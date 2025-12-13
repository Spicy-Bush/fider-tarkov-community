import { Fider } from "./fider"
import { STORAGE_KEYS, TIME } from "./constants"
import { tryLocalStorageGet, tryLocalStorageSet, tryLocalStorageRemove, tryParseJSON } from "./errors"

export interface StoredFilters {
  tags: string[]
  statuses: string[]
  myVotes: boolean
  myPosts: boolean
  notMyVotes: boolean
  date?: string
  tagLogic?: "OR" | "AND"
  view: string
  limit: number
}

export interface FilterStorageMetadata {
  timestamp: number
  wasAuthenticated: boolean
}

export const getStoredFilters = (): StoredFilters | null => {
  const stored = tryLocalStorageGet(STORAGE_KEYS.POST_FILTERS)
  if (!stored) return null

  return tryParseJSON<StoredFilters | null>(stored, null)
}

export const getFilterMetadata = (): FilterStorageMetadata | null => {
  const timestamp = tryLocalStorageGet(STORAGE_KEYS.POST_FILTERS_TIMESTAMP)
  const wasAuthenticated = tryLocalStorageGet(STORAGE_KEYS.POST_FILTERS_AUTH)

  if (!timestamp) return null

  return {
    timestamp: parseInt(timestamp, 10),
    wasAuthenticated: wasAuthenticated === "true",
  }
}

export const saveFilters = (filters: StoredFilters): void => {
  tryLocalStorageSet(STORAGE_KEYS.POST_FILTERS, JSON.stringify(filters))
  tryLocalStorageSet(STORAGE_KEYS.POST_FILTERS_TIMESTAMP, Date.now().toString())
  tryLocalStorageSet(STORAGE_KEYS.POST_FILTERS_AUTH, Fider.session.isAuthenticated ? "true" : "false")
}

export const clearFilters = (): void => {
  tryLocalStorageRemove(STORAGE_KEYS.POST_FILTERS)
  tryLocalStorageRemove(STORAGE_KEYS.POST_FILTERS_TIMESTAMP)
  tryLocalStorageRemove(STORAGE_KEYS.POST_FILTERS_AUTH)
}

export const isFilterExpired = (metadata: FilterStorageMetadata): boolean => {
  return Date.now() - metadata.timestamp > TIME.TWELVE_HOURS_MS
}

export const shouldEnableNotMyVotes = (metadata: FilterStorageMetadata | null): boolean => {
  if (!metadata) return Fider.session.isAuthenticated
  return Fider.session.isAuthenticated && !metadata.wasAuthenticated
}

export const filterStorage = {
  get: getStoredFilters,
  getMetadata: getFilterMetadata,
  save: saveFilters,
  clear: clearFilters,
  isExpired: isFilterExpired,
  shouldEnableNotMyVotes,
}

