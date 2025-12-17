import { useState, useEffect, useCallback, useRef } from "react"
import { querystring, Fider, PAGINATION, filterStorage, StoredFilters } from "@fider/services"
import { Tag } from "@fider/models"

export interface FilterState extends StoredFilters {
  query: string
}

const DEFAULT_FILTERS: FilterState = {
  tags: [],
  statuses: [],
  myVotes: false,
  myPosts: false,
  notMyVotes: false,
  date: undefined,
  tagLogic: "OR",
  query: "",
  view: "trending",
  limit: PAGINATION.DEFAULT_LIMIT,
}

export interface UsePostFiltersOptions {
  tags?: Tag[]
}

const getUrlParams = (): FilterState | null => {
  const params = new URLSearchParams(window.location.search)
  if (!params.toString()) return null

  return {
    tags: querystring.getArray("tags"),
    statuses: querystring.getArray("statuses"),
    myVotes: params.get("myvotes") === "true",
    myPosts: params.get("myposts") === "true",
    notMyVotes: params.get("notmyvotes") === "true",
    date: params.get("date") || undefined,
    tagLogic: (params.get("taglogic") as "OR" | "AND") || "OR",
    query: params.get("query") || "",
    view: params.get("view") || "trending",
    limit: Number(params.get("limit")) || PAGINATION.DEFAULT_LIMIT,
  }
}

const updateUrl = (filters: FilterState) => {
  const params = new URLSearchParams()
  if (filters.tags.length > 0) {
    filters.tags.forEach((tag) => params.append("tags", tag))
  }
  if (filters.statuses.length > 0) {
    filters.statuses.forEach((status) => params.append("statuses", status))
  }
  if (filters.myVotes) params.set("myvotes", "true")
  if (filters.myPosts) params.set("myposts", "true")
  if (filters.notMyVotes) params.set("notmyvotes", "true")
  if (filters.date) params.set("date", filters.date)
  if (filters.tagLogic !== "OR") params.set("taglogic", filters.tagLogic!)
  if (filters.query) params.set("query", filters.query)
  if (filters.view !== "trending") params.set("view", filters.view)
  if (filters.limit !== PAGINATION.DEFAULT_LIMIT) params.set("limit", filters.limit.toString())

  const newUrl = params.toString() ? `?${params.toString()}` : window.location.pathname
  window.history.replaceState({}, "", newUrl)
}

const toStoredFilters = (filters: FilterState): StoredFilters => {
  const { query, ...rest } = filters
  return rest
}

const getStoredOrDefaultFilters = (): FilterState => {
  const stored = filterStorage.get()
  const metadata = filterStorage.getMetadata()

  if (stored) {
    let restoredFilters: FilterState = { ...stored, query: "" }

    if (filterStorage.shouldEnableNotMyVotes(metadata)) {
      restoredFilters = { ...restoredFilters, notMyVotes: true }
    }

    if (metadata && filterStorage.isExpired(metadata)) {
      return { ...restoredFilters, view: "trending" }
    }

    return restoredFilters
  }

  if (Fider.session.isAuthenticated) {
    return { ...DEFAULT_FILTERS, notMyVotes: true }
  }

  return DEFAULT_FILTERS
}

export const usePostFilters = (options?: UsePostFiltersOptions) => {
  const tagsRef = useRef(options?.tags)
  tagsRef.current = options?.tags

  const isFromUrlRef = useRef(false)
  const userChangedFiltersRef = useRef(false)

  const [filters, setFilters] = useState<FilterState>(() => {
    const urlParams = getUrlParams()
    if (urlParams) {
      isFromUrlRef.current = true
      return urlParams
    }

    const storedFilters = getStoredOrDefaultFilters()
    filterStorage.save(toStoredFilters(storedFilters), tagsRef.current)
    return storedFilters
  })

  const [offset, setOffset] = useState(0)

  useEffect(() => {
    if (isFromUrlRef.current && !userChangedFiltersRef.current) {
      updateUrl(filters)
      return
    }
    filterStorage.save(toStoredFilters(filters), tagsRef.current)
    updateUrl(filters)
  }, [filters])

  const updateFilters = useCallback((newFilters: Partial<FilterState>) => {
    userChangedFiltersRef.current = true
    isFromUrlRef.current = false
    setFilters((prev) => ({ ...prev, ...newFilters }))
    setOffset(0)
  }, [])

  const resetFilters = useCallback(() => {
    userChangedFiltersRef.current = true
    isFromUrlRef.current = false
    setFilters(DEFAULT_FILTERS)
    setOffset(0)
    filterStorage.clear()
  }, [])

  const restoreSavedFilters = useCallback(() => {
    userChangedFiltersRef.current = false
    isFromUrlRef.current = false
    const storedFilters = getStoredOrDefaultFilters()
    setFilters(storedFilters)
    setOffset(0)
  }, [])

  const hasActiveFilters = useCallback(() => {
    return (
      filters.tags.length > 0 ||
      filters.statuses.length > 0 ||
      filters.myVotes ||
      filters.myPosts ||
      filters.notMyVotes ||
      !!filters.date ||
      !!filters.query ||
      filters.view !== "trending" ||
      filters.tagLogic !== "OR"
    )
  }, [filters])

  const isUsingUrlFilters = isFromUrlRef.current && !userChangedFiltersRef.current

  return {
    filters,
    offset,
    setOffset,
    updateFilters,
    resetFilters,
    restoreSavedFilters,
    hasActiveFilters,
    isUsingUrlFilters,
  }
}
