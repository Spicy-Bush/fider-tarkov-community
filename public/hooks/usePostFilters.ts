import { useState, useEffect, useCallback } from "react"
import { querystring, Fider, PAGINATION, filterStorage, StoredFilters } from "@fider/services"

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

export const usePostFilters = () => {
  const [filters, setFilters] = useState<FilterState>(() => {
    const urlParams = getUrlParams()
    if (urlParams) {
      filterStorage.save(toStoredFilters(urlParams))
      return urlParams
    }

    const stored = filterStorage.get()
    const metadata = filterStorage.getMetadata()

    if (stored) {
      let restoredFilters: FilterState = { ...stored, query: "" }

      if (filterStorage.shouldEnableNotMyVotes(metadata)) {
        restoredFilters = { ...restoredFilters, notMyVotes: true }
      }

      if (metadata && filterStorage.isExpired(metadata)) {
        const updatedFilters = { ...restoredFilters, view: "trending" }
        filterStorage.save(toStoredFilters(updatedFilters))
        return updatedFilters
      }

      filterStorage.save(toStoredFilters(restoredFilters))
      return restoredFilters
    }

    if (Fider.session.isAuthenticated) {
      return { ...DEFAULT_FILTERS, notMyVotes: true }
    }

    return DEFAULT_FILTERS
  })

  const [offset, setOffset] = useState(0)

  useEffect(() => {
    filterStorage.save(toStoredFilters(filters))
    updateUrl(filters)
  }, [filters])

  const updateFilters = useCallback((newFilters: Partial<FilterState>) => {
    setFilters((prev) => ({ ...prev, ...newFilters }))
    setOffset(0)
  }, [])

  const resetFilters = useCallback(() => {
    setFilters(DEFAULT_FILTERS)
    setOffset(0)
    filterStorage.clear()
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

  return {
    filters,
    offset,
    setOffset,
    updateFilters,
    resetFilters,
    hasActiveFilters,
  }
}
