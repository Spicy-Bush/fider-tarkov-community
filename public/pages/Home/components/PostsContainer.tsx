import "./PostsContainer.scss"

import React, { useEffect, useRef, useState } from "react"
import { Post, Tag, CurrentUser } from "@fider/models"
import { Loader, Input } from "@fider/components"
import { actions } from "@fider/services"
import IconSearch from "@fider/assets/images/heroicons-search.svg"
import IconX from "@fider/assets/images/heroicons-x.svg"
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
  const [loading, setLoading] = useState(false)
  const [posts, setPosts] = useState<Post[]>([])
  const [hasMore, setHasMore] = useState(true)
  const timerRef = useRef<number>()
  const loadMoreRef = useRef<HTMLDivElement>(null)
  const observerRef = useRef<IntersectionObserver>()
  const { filters, offset, setOffset, updateFilters, resetFilters, hasActiveFilters } = usePostFilters()

  useEffect(() => {
    searchPosts(true)
  }, [])

  useEffect(() => {
    if (!loading) {
      searchPosts(true)
    }
  }, [filters])

  useEffect(() => {
    observerRef.current = new IntersectionObserver(handleObserver, {
      root: null,
      rootMargin: "0px",
      threshold: 1.0,
    })

    if (loadMoreRef.current) {
      observerRef.current.observe(loadMoreRef.current)
    }

    return () => {
      if (observerRef.current) {
        observerRef.current.disconnect()
      }
      if (timerRef.current) {
        clearTimeout(timerRef.current)
      }
    }
  }, [])

  const handleObserver = (entries: IntersectionObserverEntry[]) => {
    const entry = entries[0]
    if (entry.isIntersecting && !loading && hasMore) {
      loadMore()
    }
  }

  const searchPosts = (reset: boolean) => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }
    setPosts(reset ? [] : posts)
    setLoading(true)
    
    timerRef.current = window.setTimeout(() => {
      const stored = localStorage.getItem("post_filters")
      const currentFilters = stored ? JSON.parse(stored) : filters
      
      actions
        .searchPosts(currentFilters)
        .then((response) => {
          if (response.ok) {
            const newPosts: Post[] = response.data || []
            const hasMore = newPosts.length === currentFilters.limit
            setPosts(newPosts)
            setHasMore(hasMore)
          }
          setLoading(false)
        })
    }, 500)
  }

  const loadMore = () => {
    const newOffset = offset + filters.limit
    setLoading(true)
    setOffset(newOffset)
    
    const stored = localStorage.getItem("post_filters")
    const currentFilters = stored ? JSON.parse(stored) : filters
    
    actions
      .searchPosts({
        ...currentFilters,
        offset: newOffset
      })
      .then((response) => {
        if (response.ok) {
          const newPosts: Post[] = response.data || []
          const hasMore = newPosts.length === currentFilters.limit
          setPosts(prev => [...prev, ...newPosts])
          setHasMore(hasMore)
        }
        setLoading(false)
      })
  }

  const handleFilterChanged = (filterState: Partial<FilterState>) => {
    updateFilters(filterState)
    searchPosts(true)
  }

  const handleSearchFilterChanged = (query: string) => {
    updateFilters({ query })
    searchPosts(true)
  }

  const handleSortChanged = (view: string) => {
    updateFilters({ view })
    searchPosts(true)
  }

  const clearSearch = () => {
    updateFilters({ query: "" })
    searchPosts(true)
  }

  const hasFilters = hasActiveFilters()
  const hasNoPosts = !loading && (!posts || posts.length === 0)
  const showResetButton = hasFilters && hasNoPosts

  return (
    <div className="c-posts-container">
      <div className="c-posts-container__header mb-5">
        {!filters.query && (
          <div className="c-posts-container__filter-col">
            <FilterPanel
              tags={[untaggedTag, ...props.tags]}
              activeFilter={filters}
              filtersChanged={handleFilterChanged}
              countPerStatus={props.countPerStatus}
            />
            <PostsSort onChange={handleSortChanged} value={filters.view || "trending"} />
          </div>
        )}
        <div className="c-posts-container__search-col">
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
      <ListPosts
        posts={posts}
        tags={props.tags}
        emptyText={i18n._("home.postscontainer.label.noresults", { message: "No results matched your search, try something different." })}
      />
      {showResetButton && (
        <div className="mt-4 text-center">
          <button 
            className="c-button c-button--default c-button--primary"
            onClick={resetFilters}
          >
            {i18n._("home.postscontainer.resetfilters", { message: "Reset all filters" })}
          </button>
        </div>
      )}
      {loading && <Loader />}
      {hasMore && <div ref={loadMoreRef} style={{ height: "1px" }}></div>}
    </div>
  )
}
