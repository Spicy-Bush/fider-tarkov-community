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

interface TagWithID {
  id: number
  slug: string
}

const VIEW_MAP: Record<string, number> = {
  "trending": 0,
  "newest": 1,
  "most-wanted": 2,
  "most-discussed": 3,
  "controversial": 4,
  "recently-updated": 5,
  "all": 6,
}

const STATUS_MAP: Record<string, number> = {
  "open": 0,
  "started": 1,
  "completed": 2,
  "declined": 3,
  "planned": 4,
  "duplicate": 5,
  "deleted": 6,
  "archived": 7,
}

function encodePFilterCookie(filters: StoredFilters, tags?: TagWithID[]): string {
  const viewNum = VIEW_MAP[filters.view] ?? 0

  // flags byte
  let flags = viewNum & 0b111
  if (filters.myVotes) flags |= 1 << 3
  if (filters.myPosts) flags |= 1 << 4
  if (filters.notMyVotes) flags |= 1 << 5

  const flagStr = String(flags)

  // statuses bitmask
  let statusB64 = ""
  if (filters.statuses && filters.statuses.length > 0) {
    let maxStatus = 0
    for (let i = 0; i < filters.statuses.length; i++) {
      const id = STATUS_MAP[filters.statuses[i]]
      if (id > maxStatus) maxStatus = id
    }
    const byteLen = ((maxStatus + 1) >> 3) + 1
    const bytes = new Uint8Array(byteLen)
    for (let i = 0; i < filters.statuses.length; i++) {
      const id = STATUS_MAP[filters.statuses[i]]
      if (id !== undefined) {
        bytes[id >> 3] |= 1 << (id & 7)
      }
    }
    statusB64 = btoa(String.fromCharCode.apply(null, bytes as unknown as number[]))
  }

  // tags bitmask
  let tagsB64 = ""
  if (tags && filters.tags.length > 0) {
    const slugToId = new Map<string, number>()
    for (let i = 0; i < tags.length; i++) {
      slugToId.set(tags[i].slug, tags[i].id)
    }

    const tagIds: number[] = []
    for (let i = 0; i < filters.tags.length; i++) {
      const slug = filters.tags[i]
      // reserve bit 0 for "untagged"
      if (slug === "untagged") {
        tagIds.push(0)
        continue
      }
      const id = slugToId.get(slug)
      if (id !== undefined) tagIds.push(id)
    }

    if (tagIds.length > 0) {
      let maxId = 0
      for (let i = 0; i < tagIds.length; i++) {
        if (tagIds[i] > maxId) maxId = tagIds[i]
      }

      const byteLen = ((maxId + 1) >> 3) + 1
      const bytes = new Uint8Array(byteLen)
      for (let i = 0; i < tagIds.length; i++) {
        const id = tagIds[i]
        bytes[id >> 3] |= 1 << (id & 7)
      }

      tagsB64 = btoa(String.fromCharCode.apply(null, bytes as unknown as number[]))
    }
  }

  return `${flagStr}:${statusB64}:${tagsB64}`
}

function setPFilterCookie(value: string): void {
  document.cookie = `pfilter=${value};path=/;max-age=31536000;SameSite=Lax`
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

export const saveFilters = (filters: StoredFilters, tags?: TagWithID[]): void => {
  tryLocalStorageSet(STORAGE_KEYS.POST_FILTERS, JSON.stringify(filters))
  tryLocalStorageSet(STORAGE_KEYS.POST_FILTERS_TIMESTAMP, Date.now().toString())
  tryLocalStorageSet(STORAGE_KEYS.POST_FILTERS_AUTH, Fider.session.isAuthenticated ? "true" : "false")
  setPFilterCookie(encodePFilterCookie(filters, tags))
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

