import React from "react"
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { Post } from "@fider/models"
import { actions, Fider, filterStorage } from "@fider/services"
import { RequestError } from "@fider/services/http"
import { savedReadingPosition, useReadingPosition } from "@fider/services/readingPosition"
import { FilterState } from "@fider/hooks/usePostFilters"
import { PostsContainer } from "./PostsContainer"
import { PostListRow } from "./ListPosts"

jest.mock("@fider/services", () => ({
  ...jest.requireActual("@fider/services/fider"),
  ...jest.requireActual("@fider/services/constants"),
  ...jest.requireActual("@fider/services/filterStorage"),
  querystring: jest.requireActual("@fider/services/querystring"),
  actions: { searchPosts: jest.fn() },
}))
jest.mock("@fider/services/readingPosition", () => ({ savedReadingPosition: jest.fn(), useReadingPosition: jest.fn() }))
jest.mock("@lingui/core", () => ({ i18n: { _: (_id: string, options: { message: string }) => options.message } }))
jest.mock("@fider/components", () => ({
  Input: ({ field, value, onChange }: any) => <input aria-label={field} value={value} onChange={(event) => onChange(event.target.value)} />,
  SwipeMode: ({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) => (
    isOpen ? <button onClick={onClose}>Close Swipe Mode</button> : null
  ),
  SwipeModeButton: ({ onClick }: { onClick: () => void }) => <button onClick={onClick}>Open Swipe Mode</button>,
}))
jest.mock("./FilterPanel", () => ({ FilterPanel: () => null }))
jest.mock("./PostsSort", () => ({
  PostsSort: ({ value, onChange }: any) => (
    <select aria-label="Order" value={value} onChange={(event) => onChange(event.target.value)}>
      <option value="trending">Trending</option>
      <option value="newest">Newest</option>
    </select>
  ),
}))
jest.mock("./ListPosts", () => ({
  ListPosts: ({ posts }: { posts: PostListRow[] }) => (
    <ul aria-label="Posts">
      {posts.map((post) => (
        <li key={post.id} data-post-id={post.id} data-post-pending={"pending" in post ? true : undefined}>
          {post.id}:{"pending" in post ? "pending" : post.title}
        </li>
      ))}
    </ul>
  ),
}))

let intersect: () => void
let observeRows: (ids: number[]) => void
let saved: {
  filters: FilterState
  rows: { id: number; height: number }[]
  visible: number[]
  nextOffset: number
  hasMore: boolean
  adHeights: Record<string, number>
  anchor?: { id: number; top: number }
} | undefined
let capture: () => unknown
const restored = jest.fn()
const cancelled = { current: false }
const initialFilters: FilterState = {
  query: "",
  view: "trending",
  tags: [],
  statuses: [],
  myVotes: false,
  myPosts: false,
  notMyVotes: false,
  tagLogic: "OR",
  limit: 20,
}

function posts(offset: number, count: number): Post[] {
  return Array.from({ length: count }, (_, index) => ({ id: offset + index + 1, title: "fresh" } as Post))
}

function showPosts(initial = posts(0, 20), criteria = initialFilters, sponsorPage?: string) {
  return render(<PostsContainer posts={initial} initialFilters={criteria} sponsorPage={sponsorPage} tags={[]} countPerStatus={{ open: 1000 }} />)
}

function shownIDs(): number[] {
  return screen.queryAllByRole("listitem").map((item) => Number(item.textContent!.split(":")[0]))
}

beforeEach(() => {
  jest.clearAllMocks()
  jest.mocked(actions.searchPosts).mockReset()
  localStorage.clear()
  document.cookie = "pfilter=;path=/;max-age=0"
  history.replaceState({}, "", "/")
  Fider.initialize({ settings: {}, tenant: { id: 1 }, props: {} })
  saved = undefined
  cancelled.current = false
  jest.mocked(savedReadingPosition).mockImplementation(() => saved)
  jest.mocked(useReadingPosition).mockImplementation((_key, read) => {
    capture = read
    return { saved, restored, cancelled } as ReturnType<typeof useReadingPosition>
  })

  global.IntersectionObserver = jest.fn((callback, options) => {
    if (options.rootMargin === "0px") {
      intersect = () => callback([{ isIntersecting: true }])
    } else {
      observeRows = (ids) => callback(ids.map((id) => ({
        target: document.querySelector(`[data-post-id="${id}"]`),
        isIntersecting: true,
      })))
    }

    return { observe: jest.fn(), disconnect: jest.fn() }
  }) as unknown as typeof IntersectionObserver
})

test.each(["response", "transport"])("a %s failure retries the same cursor and an empty page ends pagination", async (failure) => {
  if (failure === "transport") {
    jest.mocked(actions.searchPosts).mockRejectedValueOnce(new RequestError("GET", "/api/posts", "transport", new Error("offline")))
  } else {
    jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: false, status: 503, error: {} })
  }

  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(20, 15) })
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: [] })
  showPosts()

  act(intersect)
  await screen.findByRole("button", { name: "Retry" })
  expect(capture()).toMatchObject({ nextOffset: 20 })

  fireEvent.click(screen.getByRole("button", { name: "Retry" }))
  await waitFor(() => expect(shownIDs()).toHaveLength(35))
  expect(jest.mocked(actions.searchPosts).mock.calls.map(([params]) => params.offset)).toEqual([20, 20])

  act(intersect)
  await waitFor(() => expect(actions.searchPosts).toHaveBeenCalledTimes(3))
  act(intersect)

  expect(actions.searchPosts).toHaveBeenCalledTimes(3)
  expect(capture()).toMatchObject({ nextOffset: 35 })
})

test("a changed filter cancels older requests and never appends their rows", async () => {
  let finish!: (result: Awaited<ReturnType<typeof actions.searchPosts>>) => void
  jest.mocked(actions.searchPosts).mockReturnValueOnce(new Promise((resolve) => { finish = resolve }))
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(200, 2) })
  showPosts()

  act(intersect)
  const oldSignal = jest.mocked(actions.searchPosts).mock.calls[0][1]!.signal!
  fireEvent.change(screen.getByLabelText("Order"), { target: { value: "newest" } })
  await waitFor(() => expect(shownIDs()).toEqual([201, 202]))

  await act(async () => {
    finish({ ok: true, data: posts(20, 15) })
  })

  expect(oldSignal.aborted).toBe(true)
  expect(shownIDs()).toEqual([201, 202])
  expect(jest.mocked(actions.searchPosts).mock.calls[1][0]).toMatchObject({ view: "newest", offset: 0 })
})

test("a failed filter replacement cannot paginate the previous results under the new filter", async () => {
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: false, status: 503, error: {} })
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(200, 2) })
  showPosts()

  fireEvent.change(screen.getByLabelText("Order"), { target: { value: "newest" } })
  await screen.findByRole("button", { name: "Retry" })
  act(intersect)

  expect(actions.searchPosts).toHaveBeenCalledTimes(1)
  expect(capture()).toBeUndefined()

  fireEvent.click(screen.getByRole("button", { name: "Retry" }))
  await waitFor(() => expect(shownIDs()).toEqual([201, 202]))
  expect(jest.mocked(actions.searchPosts).mock.calls.map(([params]) => params.offset)).toEqual([0, 0])
})

test.each(["response", "transport"])("a %s failure after closing Swipe Mode retries the refresh before resuming pagination", async (failure) => {
  if (failure === "transport") {
    jest.mocked(actions.searchPosts).mockRejectedValueOnce(new RequestError("GET", "/api/posts", "transport", new Error("offline")))
  } else {
    jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: false, status: 503, error: {} })
  }

  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(100, 2) })
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(102, 1) })
  showPosts()

  fireEvent.click(screen.getByRole("button", { name: "Open Swipe Mode" }))
  fireEvent.click(screen.getByRole("button", { name: "Close Swipe Mode" }))
  await screen.findByRole("button", { name: "Retry" })
  act(intersect)

  expect(actions.searchPosts).toHaveBeenCalledTimes(1)
  expect(shownIDs()).toHaveLength(20)

  fireEvent.click(screen.getByRole("button", { name: "Retry" }))
  await waitFor(() => expect(actions.searchPosts).toHaveBeenCalledTimes(2))

  expect(jest.mocked(actions.searchPosts).mock.calls.map(([params]) => params.offset)).toEqual([0, 0])
  await waitFor(() => expect(shownIDs()).toEqual([101, 102]))
  expect(screen.queryByRole("alert")).toBeNull()

  act(intersect)
  await waitFor(() => expect(shownIDs()).toEqual([101, 102, 103]))
  expect(jest.mocked(actions.searchPosts).mock.calls[2][0].offset).toBe(2)
})

function savedPosition(count = 1000) {
  saved = {
    filters: initialFilters,
    rows: posts(0, count).map(({ id }) => ({ id, height: 120 })),
    visible: [501, 502, 503],
    nextOffset: count,
    hasMore: true,
    adHeights: {},
  }
}

test("restoring 1000 rows reads only visible records and discards unavailable records", async () => {
  savedPosition()
  saved!.filters = { ...initialFilters, notMyVotes: true, statuses: ["open"] }
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: [
    { ...posts(502, 1)[0], title: "updated 503" },
    { ...posts(500, 1)[0], title: "updated 501" },
  ] })
  showPosts(undefined, undefined, "current-page-grant")

  await screen.findByText("501:updated 501")
  expect(screen.getByText("503:updated 503")).toBeInTheDocument()
  expect(shownIDs()).not.toContain(502)
  expect(screen.getByText("800:pending")).toBeInTheDocument()
  expect(actions.searchPosts).toHaveBeenCalledWith({ ...saved!.filters, ids: [501, 502, 503], sponsorPage: "current-page-grant" }, {
    signal: expect.any(AbortSignal),
    notifyOnError: false,
  })
  expect(actions.searchPosts).toHaveBeenCalledTimes(1)
  expect(capture()).toMatchObject({ nextOffset: 1000 })
  expect(restored).toHaveBeenCalled()

  const snapshot = capture() as typeof saved
  expect(Object.keys(snapshot!.rows[0])).toEqual(["id", "height"])
})

test("failed visible records can be retried without reloading earlier pages", async () => {
  savedPosition()
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: false, status: 503, error: {} })
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(500, 3) })
  showPosts()

  fireEvent.click(await screen.findByRole("button", { name: "Retry" }))
  await screen.findByText("501:fresh")

  expect(jest.mocked(actions.searchPosts).mock.calls.map(([params]) => params.ids)).toEqual([[501, 502, 503], [501, 502, 503]])
  expect(restored).toHaveBeenCalled()
})

test("the initial viewport observation cannot cancel records needed by history restoration", () => {
  savedPosition()
  saved!.anchor = { id: 501, top: 20 }
  jest.mocked(actions.searchPosts).mockReturnValue(new Promise(() => {}))
  const { unmount } = showPosts()
  const signal = jest.mocked(actions.searchPosts).mock.calls[0][1]!.signal!

  act(() => observeRows([1, 2, 3]))

  expect(actions.searchPosts).toHaveBeenCalledTimes(1)
  expect(signal.aborted).toBe(false)

  unmount()
  expect(signal.aborted).toBe(true)
})

test("a changed filter aborts old record hydration and ignores its late response", async () => {
  savedPosition()
  let finish!: (result: Awaited<ReturnType<typeof actions.searchPosts>>) => void
  jest.mocked(actions.searchPosts).mockReturnValueOnce(new Promise((resolve) => { finish = resolve }))
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(2000, 2) })
  showPosts()
  const signal = jest.mocked(actions.searchPosts).mock.calls[0][1]!.signal!

  fireEvent.change(screen.getByLabelText("Order"), { target: { value: "newest" } })
  await screen.findByText("2001:fresh")
  await act(async () => { finish({ ok: true, data: posts(500, 3) }) })

  expect(signal.aborted).toBe(true)
  expect(shownIDs()).toEqual([2001, 2002])
})

test("appending a page preserves records hydrated while that page was pending", async () => {
  savedPosition()
  let finishPage!: (result: Awaited<ReturnType<typeof actions.searchPosts>>) => void
  let finishRecords!: (result: Awaited<ReturnType<typeof actions.searchPosts>>) => void
  const page = new Promise<Awaited<ReturnType<typeof actions.searchPosts>>>((resolve) => { finishPage = resolve })
  const records = new Promise<Awaited<ReturnType<typeof actions.searchPosts>>>((resolve) => { finishRecords = resolve })
  jest.mocked(actions.searchPosts).mockImplementation((params) => params.ids ? records : page)
  showPosts()
  act(intersect)

  await act(async () => { finishRecords({ ok: true, data: posts(500, 3) }) })
  await act(async () => { finishPage({ ok: true, data: posts(1000, 5) }) })

  expect(screen.getByText("501:fresh")).toBeInTheDocument()
  expect(shownIDs()).toHaveLength(1005)
  expect(capture()).toMatchObject({ nextOffset: 1005 })
})

test("changing server page caps advances by the number of rows actually consumed", async () => {
  history.replaceState({}, "", "/?limit=50")
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(20, 15) })
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(35, 5) })
  showPosts()

  act(intersect)
  await waitFor(() => expect(shownIDs()).toHaveLength(35))
  act(intersect)
  await waitFor(() => expect(shownIDs()).toHaveLength(40))

  expect(jest.mocked(actions.searchPosts).mock.calls.map(([params]) => params.offset)).toEqual([20, 35])
  expect(capture()).toMatchObject({ nextOffset: 40, hasMore: true })
})

test("an empty server list remains an authoritative empty result", () => {
  showPosts([])
  act(intersect)

  expect(actions.searchPosts).not.toHaveBeenCalled()
  expect(shownIDs()).toEqual([])
})

test("opening home with saved filters uses the server rows without a replacement request", () => {
  const selected = { ...initialFilters, view: "newest", date: "7d" }
  filterStorage.save(selected)
  showPosts(posts(200, 20), selected)

  expect(screen.getByLabelText("Order")).toHaveValue("newest")
  expect(shownIDs()).toEqual(posts(200, 20).map((post) => post.id))
  expect(actions.searchPosts).not.toHaveBeenCalled()
})

test("different initial criteria are replaced before their rows can appear", async () => {
  history.replaceState({}, "", "/?date=1d&taglogic=AND")
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(200, 2) })
  render(
    <React.StrictMode>
      <PostsContainer posts={posts(0, 20)} initialFilters={initialFilters} tags={[]} countPerStatus={{ open: 1000 }} />
    </React.StrictMode>
  )

  expect(shownIDs()).toEqual([])
  await screen.findByText("201:fresh")

  expect(jest.mocked(actions.searchPosts).mock.calls[0][0]).toMatchObject({ date: "1d", tagLogic: "AND", offset: 0 })
  expect(shownIDs()).toEqual([201, 202])
})

test("a history outline restores its own criteria despite newer saved preferences", async () => {
  savedPosition()
  localStorage.setItem("post_filters", JSON.stringify({ ...initialFilters, view: "newest" }))
  jest.mocked(actions.searchPosts).mockResolvedValueOnce({ ok: true, data: posts(500, 3) })
  showPosts()

  await screen.findByText("501:fresh")

  expect(screen.getByLabelText("Order")).toHaveValue("trending")
  expect(actions.searchPosts).toHaveBeenCalledTimes(1)
  expect(jest.mocked(actions.searchPosts).mock.calls[0][0]).toMatchObject({ view: "trending", ids: [501, 502, 503] })
})
