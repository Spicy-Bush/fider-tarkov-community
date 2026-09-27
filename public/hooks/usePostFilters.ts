import { useState, useEffect, useCallback, useRef } from "react"
import { Fider, PAGINATION, filterStorage, StoredFilters } from "@fider/services"

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
  restoredFilters?: FilterState
}

const getUrlParams = (): FilterState | null => {
  const params = new URLSearchParams(window.location.search)
  if (!params.toString()) return null

  return {
    tags: params.getAll("tags"),
    statuses: params.getAll("statuses"),
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

export const hasSamePostCriteria = (left: FilterState, right: FilterState): boolean => {
  return left.query === right.query &&
    left.view === right.view &&
    left.myVotes === right.myVotes &&
    left.myPosts === right.myPosts &&
    left.notMyVotes === right.notMyVotes &&
    (left.date || "") === (right.date || "") &&
    (left.tagLogic || "OR") === (right.tagLogic || "OR") &&
    left.tags.length === right.tags.length &&
    left.tags.every((tag, index) => tag === right.tags[index]) &&
    left.statuses.length === right.statuses.length &&
    left.statuses.every((status, index) => status === right.statuses[index])
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

  const query = params.toString()
  const newUrl = window.location.pathname + (query ? `?${query}` : "") + window.location.hash

  if (newUrl !== window.location.pathname + window.location.search + window.location.hash) {
    window.history.replaceState(window.history.state, "", newUrl)
  }
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
  const isFromUrlRef = useRef(false)
  const userChangedFiltersRef = useRef(false)

  const [filters, setFilters] = useState<FilterState>(() => {
    if (options?.restoredFilters) {
      return options.restoredFilters
    }

    const urlParams = getUrlParams()
    if (urlParams) {
      isFromUrlRef.current = true
      return urlParams
    }

    return getStoredOrDefaultFilters()
  })

  useEffect(() => {
    if (!userChangedFiltersRef.current && (isFromUrlRef.current || options?.restoredFilters)) {
      return
    }

    filterStorage.save(toStoredFilters(filters))

    if (userChangedFiltersRef.current) {
      updateUrl(filters)
    }
  }, [filters])

  const updateFilters = useCallback((newFilters: Partial<FilterState>) => {
    userChangedFiltersRef.current = true
    isFromUrlRef.current = false
    setFilters((prev) => ({ ...prev, ...newFilters }))
  }, [])

  const resetFilters = useCallback(() => {
    userChangedFiltersRef.current = true
    isFromUrlRef.current = false
    setFilters(DEFAULT_FILTERS)
    filterStorage.clear()
  }, [])

  const restoreSavedFilters = useCallback(() => {
    userChangedFiltersRef.current = false
    isFromUrlRef.current = false
    const storedFilters = getStoredOrDefaultFilters()
    updateUrl(storedFilters)
    setFilters(storedFilters)
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
    updateFilters,
    resetFilters,
    restoreSavedFilters,
    hasActiveFilters,
    isUsingUrlFilters,
  }
}
