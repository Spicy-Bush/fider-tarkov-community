import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react"
import { Post, Tag, CurrentUser } from "@fider/models"
import { Input, SwipeMode, SwipeModeButton } from "@fider/components"
import { actions } from "@fider/services"
import { heroiconsSearch as IconSearch, heroiconsX as IconX } from "@fider/icons.generated"
import { FilterPanel } from "./FilterPanel"
import { ListPosts, PostListRow } from "./ListPosts"
import { i18n } from "@lingui/core"
import { PostsSort } from "./PostsSort"
import { usePostFilters, FilterState, hasSamePostCriteria } from "@fider/hooks/usePostFilters"
import { RequestError } from "@fider/services/http"
import { savedReadingPosition, useReadingPosition } from "@fider/services/readingPosition"

interface PostsContainerProps {
  user?: CurrentUser
  posts: Post[]
  tags: Tag[]
  countPerStatus: { [key: string]: number }
  initialFilters: FilterState
  savedFiltersAt?: number
}

interface PostPosition {
  filters: FilterState
  rows: { id: number; height: number }[]
  visible: number[]
  nextOffset: number
  hasMore: boolean
  anchor?: { id: number; top: number }
  adHeights: Record<string, number>
}

interface PostList {
  filters: FilterState
  posts: PostListRow[]
  nextOffset: number
  hasMore: boolean
}

type PostLoadMode = "replace" | "append"

type PostLoadState =
  | { status: "pending"; mode: PostLoadMode }
  | { status: "failed"; mode: PostLoadMode; message: string }

const untaggedTag: Tag = {
  permissions: { assign: false },
  id: -1,
  slug: "untagged",
  name: "untagged",
  color: "cccccc",
  isPublic: false,
}

export const PostsContainer: React.FC<PostsContainerProps> = (props) => {
  const position = useRef(savedReadingPosition<PostPosition>("home-posts"))
  const { filters, updateFilters, resetFilters, hasActiveFilters } = usePostFilters({
    restoredFilters: position.current?.filters,
    initialFilters: props.initialFilters,
    savedFiltersAt: props.savedFiltersAt,
    tags: props.tags,
  })
  const initialMatches = hasSamePostCriteria(props.initialFilters, filters)
  const initialPosts = initialMatches ? props.posts || [] : []
  const container = useRef<HTMLDivElement>(null)
  const reading = useReadingPosition<PostPosition | undefined>("home-posts", (): PostPosition | undefined => {
    if (list.filters !== filters) return undefined

    const rows = [...(container.current?.querySelectorAll<HTMLElement>("[data-post-id]") ?? [])].map((element) => ({
      id: Number(element.dataset.postId),
      rect: element.getBoundingClientRect(),
    }))
    const anchor = rows.find(({ rect }) => rect.bottom > 0 && rect.top < window.innerHeight)

    return {
      filters,
      rows: rows.map(({ id, rect }) => ({ id, height: rect.height })),
      visible: rows.filter(({ rect }) => rect.bottom > -400 && rect.top < window.innerHeight + 400).map(({ id }) => id),
      nextOffset: list.nextOffset,
      hasMore: list.hasMore,
      anchor: anchor ? { id: anchor.id, top: anchor.rect.top } : undefined,
      adHeights: Object.fromEntries([...(container.current?.querySelectorAll<HTMLElement>("[data-feed-slot]") ?? [])].map((element) => [
        element.dataset.feedSlot!,
        element.getBoundingClientRect().height,
      ])),
    }
  })
  const [list, setList] = useState<PostList>(() => {
    const records = new Map(initialPosts.map((post) => [post.id, post]))
    const posts: PostListRow[] = reading.saved?.rows.map((row) => records.get(row.id) ?? { ...row, pending: true }) ?? initialPosts

    return {
      filters,
      posts,
      nextOffset: reading.saved?.nextOffset ?? initialPosts.length,
      hasMore: reading.saved?.hasMore ?? initialPosts.length > 0,
    }
  })
  const [loadState, setLoadState] = useState<PostLoadState | undefined>(
    !initialMatches && !reading.saved ? { status: "pending", mode: "replace" } : undefined
  )
  const loading = loadState?.status === "pending" ? loadState.mode : null
  const failedLoad = loadState?.status === "failed" ? loadState : undefined
  const [visible, setVisible] = useState(() => [...(reading.saved?.visible ?? [])].sort((a, b) => a - b))
  const [recordsError, setRecordsError] = useState<string>()
  const readingDone = useRef(false)
  const [isSwipeModeOpen, setIsSwipeModeOpen] = useState(false)
  const timerRef = useRef<number>()
  const loadMoreRef = useRef<HTMLDivElement>(null)
  const request = useRef<AbortController>()
  const { posts, hasMore } = list

  const loadPosts = useCallback(async (mode: PostLoadMode) => {
    if (mode === "append" && (list.filters !== filters || (request.current && !request.current.signal.aborted))) {
      return
    }

    request.current?.abort()
    const current = new AbortController()
    request.current = current
    setLoadState({ status: "pending", mode })

    const previous = mode === "append" ? list : { filters, posts: [], nextOffset: 0, hasMore: true }
    let failure: string | undefined

    try {
      const response = await actions.searchPosts({ ...filters, offset: previous.nextOffset }, {
        signal: current.signal,
        notifyOnError: false,
      })
      if (current.signal.aborted) return

      if (!response.ok) {
        failure = response.error.errors?.[0]?.message || "Could not load posts. Please try again."
        return
      }

      const received = response.data || []
      setList((currentList) => {
        const merged = new Map(mode === "append" ? currentList.posts.map((post) => [post.id, post]) : [])

        for (const post of received) {
          merged.set(post.id, post)
        }

        return {
          filters,
          posts: [...merged.values()],
          nextOffset: previous.nextOffset + received.length,
          hasMore: received.length > 0,
        }
      })
    } catch (cause) {
      if (current.signal.aborted) return
      if (!(cause instanceof RequestError)) throw cause

      failure = "Could not load posts. Please try again."
    } finally {
      if (request.current === current) {
        request.current = undefined
        setLoadState(failure ? { status: "failed", mode, message: failure } : undefined)
      }

      current.abort()
    }
  }, [filters, list])

  useEffect(() => {
    const observer = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        const id = Number((entry.target as HTMLElement).dataset.postId)

        if (entry.isIntersecting) {
          observed.add(id)
        } else {
          observed.delete(id)
        }
      }

      if (!readingDone.current && reading.saved?.anchor && !reading.cancelled.current) {
        return
      }

      const next = [...observed].sort((a, b) => a - b)
      setVisible((previous) => {
        const unchanged = previous.length === next.length && previous.every((id, index) => id === next[index])
        return unchanged ? previous : next
      })
    }, { rootMargin: "400px" })
    const observed = new Set<number>()

    for (const element of container.current?.querySelectorAll("[data-post-id]") ?? []) {
      observer.observe(element)
    }

    return () => observer.disconnect()
  }, [list.posts])

  useEffect(() => {
    if (recordsError || list.filters !== filters) return

    const pending = new Set(list.posts.filter((post) => "pending" in post).map((post) => post.id))
    const ids = visible.filter((id) => pending.has(id)).slice(0, 50)
    if (ids.length === 0) return

    const current = new AbortController()
    const hydrate = async () => {
      try {
        const result = await actions.searchPosts({ ...filters, ids }, {
          signal: current.signal,
          notifyOnError: false,
        })
        if (current.signal.aborted) return

        if (!result.ok) {
          setRecordsError("Could not load these posts. Please try again.")
          return
        }

        const requested = new Set(ids)
        const found = new Map((result.data || []).map((post) => [post.id, post]))
        setList((previous) => ({
          ...previous,
          posts: previous.posts.flatMap((post) => {
            if (!("pending" in post) || !requested.has(post.id)) return [post]

            const record = found.get(post.id)
            return record ? [record] : []
          }),
        }))
      } catch (cause) {
        if (current.signal.aborted) return
        if (!(cause instanceof RequestError)) throw cause

        setRecordsError("Could not load these posts. Please try again.")
      }
    }

    void hydrate()
    return () => current.abort()
  }, [filters, list.filters, list.posts, visible, recordsError])

  useLayoutEffect(() => {
    if (readingDone.current) {
      reading.restored()
      return
    }

    const anchor = reading.saved?.anchor
    if (!anchor || reading.cancelled.current || recordsError) {
      readingDone.current = true
      reading.restored()
      return
    }

    if (list.posts.some((post) => "pending" in post && visible.includes(post.id))) return

    const frame = requestAnimationFrame(() => {
      if (!reading.cancelled.current) {
        const element = container.current?.querySelector<HTMLElement>(`[data-post-id="${anchor.id}"]`)

        if (element) {
          window.scrollBy({ top: element.getBoundingClientRect().top - anchor.top, behavior: "instant" })
        }
      }

      readingDone.current = true
      reading.restored()
    })

    return () => cancelAnimationFrame(frame)
  }, [reading.saved, reading.restored, reading.cancelled, list.posts, visible, recordsError])

  const handleFilterChanged = useCallback((filterState: Partial<FilterState>) => {
    updateFilters(filterState)
  }, [updateFilters])

  const handleSearchFilterChanged = useCallback((query: string) => {
    updateFilters({ query })
  }, [updateFilters])

  const handleSortChanged = useCallback((view: string) => {
    updateFilters({ view })
  }, [updateFilters])

  const clearSearch = useCallback(() => {
    updateFilters({ query: "" })
  }, [updateFilters])

  const previousFilters = useRef(!initialMatches && !reading.saved ? undefined : filters)

  useEffect(() => {
    if (previousFilters.current === filters) return

    const debounce = previousFilters.current !== undefined && previousFilters.current.query !== filters.query
    request.current?.abort()
    request.current = undefined
    clearTimeout(timerRef.current)
    setLoadState({ status: "pending", mode: "replace" })
    setRecordsError(undefined)
    timerRef.current = window.setTimeout(() => {
      previousFilters.current = filters
      void loadPosts("replace")
    }, debounce ? 300 : 0)
  }, [filters, loadPosts])

  useEffect(() => {
    return () => {
      clearTimeout(timerRef.current)
      request.current?.abort()
    }
  }, [])

  useEffect(() => {
    const handleObserver = (entries: IntersectionObserverEntry[]) => {
      const entry = entries[0]
      if (entry.isIntersecting && !loading && !failedLoad && hasMore) {
        void loadPosts("append")
      }
    }

    const observer = new IntersectionObserver(handleObserver, {
      root: null,
      rootMargin: "0px",
      threshold: 1.0,
    })

    if (loadMoreRef.current) {
      observer.observe(loadMoreRef.current)
    }

    return () => {
      observer.disconnect()
    }
  }, [loading, failedLoad, hasMore, loadPosts])

  const hasFilters = hasActiveFilters()
  const hasNoPosts = !loading && (!posts || posts.length === 0)
  const showResetButton = hasFilters && hasNoPosts

  return (
    <div>
      <div className="flex flex-col gap-3 mb-5">
        <div className="flex items-center gap-2">
          {!filters.query && (
            <>
              <FilterPanel
                tags={[untaggedTag, ...props.tags]}
                activeFilter={filters}
                filtersChanged={handleFilterChanged}
                countPerStatus={props.countPerStatus}
              />
              <PostsSort onChange={handleSortChanged} value={filters.view || "trending"} />
              <SwipeModeButton onClick={() => setIsSwipeModeOpen(true)} />
            </>
          )}
          <div className={`${filters.query ? 'w-full' : 'ml-auto w-[200px] max-md:hidden'}`}>
            <Input
              field="query"
              icon={filters.query ? IconX : IconSearch}
              onIconClick={filters.query ? clearSearch : undefined}
              placeholder={i18n._("home.postscontainer.query.placeholder", { message: "Search" })}
              value={filters.query}
              onChange={handleSearchFilterChanged}
            />
          </div>
        </div>
        {!filters.query && (
          <div className="md:hidden">
            <Input
              field="query-mobile"
              icon={IconSearch}
              placeholder={i18n._("home.postscontainer.query.placeholder", { message: "Search" })}
              value={filters.query}
              onChange={handleSearchFilterChanged}
            />
          </div>
        )}
      </div>
      <div
        ref={container}
        className={loading === "replace" && posts.length > 0 ? "opacity-60 transition-opacity duration-150 delay-100" : "transition-opacity duration-75"}
        aria-busy={loading !== null}
      >
        {recordsError && (
          <div role="alert" className="sticky top-4 z-10 mb-4 flex items-center justify-center gap-3 rounded-card bg-elevated p-3 text-sm shadow">
            <span>{recordsError}</span>
            <button
              className="cursor-pointer font-medium text-primary hover:underline"
              onClick={() => {
                setRecordsError(undefined)
              }}
            >
              Retry
            </button>
          </div>
        )}
        <ListPosts
          posts={posts}
          tags={props.tags}
          loading={loading === "replace" && posts.length === 0}
          insertFeedAds
          feedAdHeights={reading.saved?.adHeights}
          emptyText={i18n._("home.postscontainer.label.noresults", { message: "No results matched your search, try something different." })}
        />
      </div>
      {showResetButton && (
        <div className="mt-4 text-center">
          <button 
            className="inline-flex items-center px-4 py-2 bg-primary text-white rounded cursor-pointer"
            onClick={resetFilters}
          >
            {i18n._("home.postscontainer.resetfilters", { message: "Reset all filters" })}
          </button>
        </div>
      )}
      {failedLoad && (
        <div role="alert" className="mt-4 flex items-center justify-center gap-3 text-sm">
          <span>{failedLoad.message}</span>
          <button
            className="cursor-pointer font-medium text-primary hover:underline"
            onClick={() => void loadPosts(failedLoad.mode)}
          >
            Retry
          </button>
        </div>
      )}
      {loading === "append" && (
        <div className="mt-4 text-center">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
        </div>
      )}
      {hasMore && <div ref={loadMoreRef} className="h-px"></div>}
      <SwipeMode
        tags={props.tags}
        isOpen={isSwipeModeOpen}
        onClose={() => {
          setIsSwipeModeOpen(false)
          void loadPosts("replace")
        }}
      />
    </div>
  )
}
