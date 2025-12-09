import { useState, useEffect, useCallback } from "react"
import { querystring, Fider } from "@fider/services"

export interface FilterState {
  tags: string[]
  statuses: string[]
  myVotes: boolean
  myPosts: boolean
  notMyVotes: boolean
  date?: string
  tagLogic?: "OR" | "AND"
  query: string
  view: string
  limit: number
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
  limit: 15
}

const STORAGE_KEY = "post_filters"
const FILTER_TIMESTAMP_KEY = "post_filters_timestamp"
const AUTH_STATE_KEY = "post_filters_auth"
const TWELVE_HOURS_MS = 12 * 60 * 60 * 1000

const getUrlParams = () => {
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
    limit: Number(params.get("limit")) || 15
  }
}

const updateUrl = (filters: FilterState) => {
  const params = new URLSearchParams()
  if (filters.tags.length > 0) {
    filters.tags.forEach(tag => params.append("tags", tag))
  }
  if (filters.statuses.length > 0) {
    filters.statuses.forEach(status => params.append("statuses", status))
  }
  if (filters.myVotes) params.set("myvotes", "true")
  if (filters.myPosts) params.set("myposts", "true")
  if (filters.notMyVotes) params.set("notmyvotes", "true")
  if (filters.date) params.set("date", filters.date)
  if (filters.tagLogic !== "OR") params.set("taglogic", filters.tagLogic!)
  if (filters.query) params.set("query", filters.query)
  if (filters.view !== "trending") params.set("view", filters.view)
  if (filters.limit !== 15) params.set("limit", filters.limit.toString())

  const newUrl = params.toString() ? `?${params.toString()}` : window.location.pathname
  window.history.replaceState({}, "", newUrl)
}

const saveToLocalStorage = (filters: FilterState) => {
  const filtersToSave = { ...filters, query: "" }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(filtersToSave))
  localStorage.setItem(FILTER_TIMESTAMP_KEY, Date.now().toString())
  localStorage.setItem(AUTH_STATE_KEY, Fider.session.isAuthenticated ? "true" : "false")
}

export const usePostFilters = () => {
  const [filters, setFilters] = useState<FilterState>(() => {
    const urlParams = getUrlParams()
    if (urlParams) {
      saveToLocalStorage(urlParams)
      return urlParams
    }

    const stored = localStorage.getItem(STORAGE_KEY)
    const timestamp = localStorage.getItem(FILTER_TIMESTAMP_KEY)
    const wasAuthenticated = localStorage.getItem(AUTH_STATE_KEY) === "true"
    const now = Date.now()
    
    if (stored) {
      try {
        const parsedFilters = JSON.parse(stored)
        let restoredFilters = { ...parsedFilters, query: "" }
        
        if (Fider.session.isAuthenticated && !wasAuthenticated) {
          restoredFilters = { ...restoredFilters, notMyVotes: true }
        }
        
        if (timestamp && (now - parseInt(timestamp)) > TWELVE_HOURS_MS) {
          const updatedFilters = { ...restoredFilters, view: "trending" }
          saveToLocalStorage(updatedFilters)
          return updatedFilters
        }
        saveToLocalStorage(restoredFilters)
        return restoredFilters
      } catch {
        return DEFAULT_FILTERS
      }
    }

    if (Fider.session.isAuthenticated) {
      return { ...DEFAULT_FILTERS, notMyVotes: true }
    }

    return DEFAULT_FILTERS
  })

  const [offset, setOffset] = useState(0)

  useEffect(() => {
    saveToLocalStorage(filters)
    updateUrl(filters)
  }, [filters])

  const updateFilters = useCallback((newFilters: Partial<FilterState>) => {
    setFilters(prev => ({ ...prev, ...newFilters }))
    setOffset(0)
  }, [])

  const resetFilters = useCallback(() => {
    setFilters(DEFAULT_FILTERS)
    setOffset(0)
    localStorage.removeItem(STORAGE_KEY)
    localStorage.removeItem(FILTER_TIMESTAMP_KEY)
    localStorage.removeItem(AUTH_STATE_KEY)
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
    hasActiveFilters
  }
} 