import "./PostsContainer.scss"

import React, { useEffect, useRef, useState, useCallback } from "react"
import { Post, Tag, CurrentUser } from "@fider/models"
import { Input } from "@fider/components"
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
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [posts, setPosts] = useState<Post[]>([])
  const [hasMore, setHasMore] = useState(true)
  const timerRef = useRef<number>()
  const loadMoreRef = useRef<HTMLDivElement>(null)
  const seenPostIds = useRef(new Set<number>())
  const { filters, offset, setOffset, updateFilters, resetFilters, hasActiveFilters } = usePostFilters()

  const searchPosts = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }
    setLoading(true)
    setOffset(0)
    seenPostIds.current.clear()
    
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

  useEffect(() => {
    searchPosts()
  }, [])

  useEffect(() => {
    if (!loading) {
      searchPosts()
    }
  }, [filters])

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
      if (timerRef.current) {
        clearTimeout(timerRef.current)
      }
    }
  }, [loading, loadingMore, hasMore, loadMore])

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
        loading={loading}
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
      {loadingMore && (
        <div className="mt-4 text-center">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-base"></div>
        </div>
      )}
      {hasMore && <div ref={loadMoreRef} style={{ height: "1px" }}></div>}
    </div>
  )
}
