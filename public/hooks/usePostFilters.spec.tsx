import { act, renderHook } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { Fider, filterStorage } from "@fider/services"
import { FilterState, hasSamePostCriteria, usePostFilters } from "./usePostFilters"

jest.mock("@fider/services", () => ({
  ...jest.requireActual("@fider/services/fider"),
  ...jest.requireActual("@fider/services/constants"),
  ...jest.requireActual("@fider/services/filterStorage"),
}))

beforeEach(() => {
  jest.restoreAllMocks()
  localStorage.clear()
  document.cookie = "pfilter=;path=/;max-age=0"
  window.history.replaceState(null, "", "/")
  Fider.initialize({ settings: {}, tenant: { id: 1 }, props: {} })
})

test("shared filter URLs retain every selected tag and status", () => {
  window.history.replaceState(null, "", "/?tags=alpha&tags=beta%26gamma&statuses=open&statuses=planned&view=newest")
  const { result } = renderHook(() => usePostFilters())

  expect(result.current.filters).toMatchObject({
    tags: ["alpha", "beta&gamma"],
    statuses: ["open", "planned"],
    view: "newest",
  })
  expect(filterStorage.get()).toBeNull()
})

test("reopening a URL written by the filter controls preserves the selection", () => {
  const first = renderHook(() => usePostFilters())

  act(() => first.result.current.updateFilters({
    tags: ["alpha", "beta"],
    statuses: ["open", "completed"],
    date: "7d",
    tagLogic: "AND",
  }))

  const selected = first.result.current.filters
  first.unmount()
  const reopened = renderHook(() => usePostFilters())

  expect(reopened.result.current.filters).toEqual(selected)
})

const preferences: FilterState = {
  tags: Array.from({ length: 100 }, (_, index) => `a-meaningful-tag-slug-number-${index}`),
  statuses: ["open", "planned"],
  myVotes: false,
  myPosts: false,
  notMyVotes: false,
  date: "7d",
  tagLogic: "AND",
  query: "",
  view: "newest",
  limit: 15,
}

const tags = preferences.tags.map((slug, index) => ({ id: 1_000_000 + index, slug }))

test("saved filters fit in a cookie even when selected tags have sparse IDs and long slugs", () => {
  filterStorage.save(preferences, tags)
  const { result } = renderHook(() => usePostFilters({ tags }))

  expect(result.current.filters).toEqual(preferences)
  const value = document.cookie.split("; ").find((cookie) => cookie.startsWith("pfilter="))!.slice(8)
  expect(value.length).toBeLessThan(2000)
  expect(JSON.parse(decodeURIComponent(value))).toMatchObject({
    tagIds: tags.map((tag) => tag.id),
    date: "7d",
    tagLogic: "AND",
    view: "newest",
    statuses: ["open", "planned"],
  })
})

test("the server's saved selection owns initial rendering, including expired sorting", () => {
  filterStorage.save(preferences, tags)
  const initialFilters = { ...preferences, view: "trending" }
  const savedFiltersAt = filterStorage.getMetadata()!.timestamp
  const { result } = renderHook(() => usePostFilters({ initialFilters, savedFiltersAt, tags }))

  expect(result.current.filters).toEqual(initialFilters)
  expect(new URLSearchParams(window.location.search).get("view")).toBeNull()
  expect(new URLSearchParams(window.location.search).get("date")).toBe("7d")
})

test.each(["rejected cookie", "older accepted cookie"])("%s preserves the newer local selection", (kind) => {
  filterStorage.save(preferences, tags)
  const timestamp = filterStorage.getMetadata()!.timestamp
  document.cookie = "pfilter=%7B%7D;path=/"
  const initialFilters = { ...preferences, tags: [], statuses: [], view: "trending", date: "", tagLogic: "OR" as const }
  const savedFiltersAt = kind === "rejected cookie" ? 0 : timestamp - 1
  const { result } = renderHook(() => usePostFilters({ initialFilters, savedFiltersAt, tags }))

  expect(result.current.filters).toEqual(preferences)
})

test("expired sorting resets while the remaining saved selection survives", () => {
  filterStorage.save(preferences)
  jest.spyOn(Date, "now").mockReturnValue(Date.now() + 13 * 60 * 60 * 1000)
  const { result } = renderHook(() => usePostFilters())

  expect(result.current.filters).toEqual({ ...preferences, view: "trending" })
})

test("signing in enables unvoted posts without discarding saved selection", () => {
  filterStorage.save(preferences)
  Fider.initialize({ settings: {}, tenant: { id: 1 }, props: {}, user: { id: 7 } })
  const { result } = renderHook(() => usePostFilters())

  expect(result.current.filters).toEqual({ ...preferences, notMyVotes: true })
})

test("history restores its own selection after saved preferences change", () => {
  const newer = { ...preferences, view: "most-wanted", date: "1d" }
  filterStorage.save(newer)
  const { result } = renderHook(() => usePostFilters({ restoredFilters: preferences }))

  expect(result.current.filters).toEqual(preferences)
  expect(filterStorage.get()).toEqual(newer)

  act(() => result.current.updateFilters({ date: "30d" }))

  expect(filterStorage.get()).toMatchObject({ date: "30d", view: preferences.view })
})

test("page sizes do not change query criteria, while date and tag matching do", () => {
  expect(hasSamePostCriteria(preferences, { ...preferences, limit: 20 })).toBe(true)
  expect(hasSamePostCriteria(preferences, { ...preferences, date: undefined })).toBe(false)
  expect(hasSamePostCriteria(preferences, { ...preferences, tagLogic: "OR" })).toBe(false)
})
