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

test("saved preferences retain large selections, dates, and tag matching without cookie limits", () => {
  filterStorage.save(preferences)
  const { result } = renderHook(() => usePostFilters())

  expect(result.current.filters).toEqual(preferences)
  expect(document.cookie).not.toContain("pfilter=")
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
