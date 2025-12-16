// import "./PostsContainer.scss"

import React, { useEffect, useRef, useState, useCallback } from "react"
import { Post, Tag, CurrentUser } from "@fider/models"
import { Input, SwipeMode, SwipeModeButton } from "@fider/components"
import { actions } from "@fider/services"
import { heroiconsSearch as IconSearch, heroiconsX as IconX } from "@fider/icons.generated"
import { FilterPanel } from "./FilterPanel"
import { ListPosts } from "./ListPosts"
import { i18n } from "@lingui/core"
import { PostsSort } from "./PostsSort"
import { usePostFilters, FilterState } from "@fider/hooks/usePostFilters"

interface PostsContainerProps {
  user?: CurrentUser
  posts: Post[]
  tags: Tag[]
  countPerStatus: { [key: string]: number }
}

const untaggedTag: Tag = {
  id: -1,
  slug: "untagged",
  name: "untagged",
  color: "cccccc",
  isPublic: false,
}

export const PostsContainer: React.FC<PostsContainerProps> = (props) => {
  const initialPosts = props.posts || []
  const hasInitialPosts = initialPosts.length > 0
  
  const [loading, setLoading] = useState(!hasInitialPosts)
  const [loadingMore, setLoadingMore] = useState(false)
  const [posts, setPosts] = useState<Post[]>(initialPosts)
  const [hasMore, setHasMore] = useState(hasInitialPosts && initialPosts.length >= 20)
  const [isSwipeModeOpen, setIsSwipeModeOpen] = useState(false)
  const timerRef = useRef<number>()
  const loadMoreRef = useRef<HTMLDivElement>(null)
  const seenPostIds = useRef(new Set<number>())
  const { filters, offset, setOffset, updateFilters, resetFilters, hasActiveFilters } = usePostFilters({ tags: props.tags })
  
  useEffect(() => {
    if (hasInitialPosts) {
      for (let i = 0; i < initialPosts.length; i++) {
        seenPostIds.current.add(initialPosts[i].id)
      }
    }
  }, [])

  const searchPosts = useCallback((resetOffset = true) => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }
    setLoading(true)
    if (resetOffset) {
      setOffset(0)
      seenPostIds.current.clear()
    }
    
    timerRef.current = window.setTimeout(() => {
      actions
        .searchPosts(filters)
        .then((response) => {
          if (response.ok) {
            const newPosts: Post[] = response.data || []
            const hasMore = newPosts.length === filters.limit
            
            for (let i = 0; i < newPosts.length; i++) {
              seenPostIds.current.add(newPosts[i].id)
            }
            
            setPosts(newPosts)
            setHasMore(hasMore)
          }
          setLoading(false)
        })
    }, 500)
  }, [filters, setOffset])

  const loadMore = useCallback(() => {
    const newOffset = offset + filters.limit
    setLoadingMore(true)
    setOffset(newOffset)
    
    actions
      .searchPosts({
        ...filters,
        offset: newOffset
      })
      .then((response) => {
        if (response.ok) {
          const newPosts: Post[] = response.data || []
          const hasMore = newPosts.length === filters.limit
          
          const uniqueNewPosts: Post[] = []
          for (let i = 0; i < newPosts.length; i++) {
            const post = newPosts[i]
            if (!seenPostIds.current.has(post.id)) {
              uniqueNewPosts.push(post)
              seenPostIds.current.add(post.id)
            }
          }
          
          setPosts(prev => prev.concat(uniqueNewPosts))
          setHasMore(hasMore)
        }
        setLoadingMore(false)
      })
  }, [offset, filters, setOffset])

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

  const prevFiltersRef = useRef(filters)

  const arraysEqual = (a: string[] = [], b: string[] = []) => {
    if (a.length !== b.length) return false
    for (let i = 0; i < a.length; i++) {
      if (a[i] !== b[i]) return false
    }
    return true
  }

  useEffect(() => {
    const prev = prevFiltersRef.current
    const filtersChanged =
      prev.view !== filters.view ||
      !arraysEqual(prev.tags, filters.tags) ||
      !arraysEqual(prev.statuses, filters.statuses) ||
      prev.query !== filters.query ||
      prev.myVotes !== filters.myVotes ||
      prev.myPosts !== filters.myPosts ||
      prev.notMyVotes !== filters.notMyVotes ||
      prev.date !== filters.date ||
      prev.tagLogic !== filters.tagLogic ||
      prev.limit !== filters.limit

    prevFiltersRef.current = filters

    if (filtersChanged && offset === 0) {
      searchPosts(true)
    }
  }, [filters, offset, searchPosts])

  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
      }
    }
  }, [])

  useEffect(() => {
    const handleObserver = (entries: IntersectionObserverEntry[]) => {
      const entry = entries[0]
      if (entry.isIntersecting && !loading && !loadingMore && hasMore) {
        loadMore()
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
  }, [loading, loadingMore, hasMore, loadMore])

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
      <ListPosts
        posts={posts}
        tags={props.tags}
        loading={loading}
        emptyText={i18n._("home.postscontainer.label.noresults", { message: "No results matched your search, try something different." })}
      />
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
      {loadingMore && (
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
          searchPosts()
        }}
      />
    </div>
  )
}
