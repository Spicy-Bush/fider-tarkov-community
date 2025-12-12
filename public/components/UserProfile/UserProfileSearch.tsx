// UserProfileSearch converted to Tailwind

import React, { useState, useEffect, useRef, useCallback } from "react"
import { Input, Select, Icon, Loader } from "@fider/components"
import { useUserProfile } from "./context"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { actions } from "@fider/services"
import { heroiconsSearch as IconSearch, heroiconsX as IconX, heroiconsArrowUpDown as IconSort, heroiconsChevronUp as IconChevronUp } from "@fider/icons.generated"

type ContentType = "all" | "posts" | "comments" | "voted"
type SortField = "createdAt" | "title"
type SortOrder = "asc" | "desc"

const parseMentions = (text: string): React.ReactNode => {
  const parts: React.ReactNode[] = []
  const regex = /@\{[^}]+\}/g
  let lastIndex = 0
  let match
  let key = 0

  while ((match = regex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      parts.push(text.slice(lastIndex, match.index))
    }

    try {
      const json = match[0].substring(1)
      const mention = JSON.parse(json)
      parts.push(
        <span key={key++} className="text-primary font-medium">@{mention.name}</span>
      )
    } catch {
      parts.push(match[0])
    }

    lastIndex = match.index + match[0].length
  }

  if (lastIndex < text.length) {
    parts.push(text.slice(lastIndex))
  }

  return parts.length > 0 ? parts : text
}

interface Post {
  id: number
  title: string
  createdAt: string
}

interface Comment {
  id: number
  postNumber: number
  postTitle: string
  content: string
  createdAt: string
}

interface SearchResults {
  posts: Post[]
  comments: Comment[]
}

const UserProfileSearchComponent: React.FC = () => {
  const { user, activeTab } = useUserProfile()
  const [searchQuery, setSearchQuery] = useState("")
  const [contentType, setContentType] = useState<ContentType>("all")
  const [voteType, setVoteType] = useState<"up" | "down" | undefined>(undefined)
  const [sortField, setSortField] = useState<SortField>("createdAt")
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc")
  const [isLoading, setIsLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [hasMore, setHasMore] = useState(true)
  const [showBackToTop, setShowBackToTop] = useState(false)
  const [searchResults, setSearchResults] = useState<SearchResults>({ posts: [], comments: [] })
  const timerRef = useRef<number>()
  const loadMoreRef = useRef<HTMLDivElement>(null)
  const stateRef = useRef({ loading: false, hasMore: true, results: searchResults, contentType, activeTab })

  const loadSearchResults = useCallback(async (offset: number) => {
    if (!user) return

    const options = {
      contentType: contentType === "all" ? "" : contentType,
      voteType: contentType === "voted" ? voteType : undefined,
      limit: 10,
      offset,
      sortBy: sortField,
      sortOrder,
    }

    const result = await actions.searchUserContent(user.id, searchQuery, options)
    if (result.ok) {
      if (offset === 0) {
        setSearchResults(result.data)
        stateRef.current.results = result.data
      } else {
        setSearchResults(prev => {
          const newResults = {
            posts: [...prev.posts, ...result.data.posts],
            comments: [...prev.comments, ...result.data.comments],
          }
          stateRef.current.results = newResults
          return newResults
        })
      }
      const hasMorePosts = result.data.posts.length === options.limit
      const hasMoreComments = result.data.comments.length === options.limit
      const newHasMore = hasMorePosts || hasMoreComments
      setHasMore(newHasMore)
      stateRef.current.hasMore = newHasMore
    }
    setIsLoading(false)
    setLoadingMore(false)
    stateRef.current.loading = false

    requestAnimationFrame(() => {
      const sentinel = loadMoreRef.current
      if (sentinel && stateRef.current.hasMore) {
        const rect = sentinel.getBoundingClientRect()
        if (rect.top < window.innerHeight + 200) {
          stateRef.current.loading = true
          setLoadingMore(true)
          loadSearchResults(getNextOffset())
        }
      }
    })
  }, [user, contentType, voteType, sortField, sortOrder, searchQuery])

  const getNextOffset = () => {
    const { results, contentType: type } = stateRef.current
    if (type === "all") {
      return Math.max(results.posts.length, results.comments.length)
    } else if (type === "posts" || type === "voted") {
      return results.posts.length
    }
    return results.comments.length
  }

  useEffect(() => {
    stateRef.current.contentType = contentType
    stateRef.current.activeTab = activeTab
  }, [contentType, activeTab])

  useEffect(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }

    if (activeTab !== "search") return

    setHasMore(true)
    stateRef.current = { ...stateRef.current, hasMore: true, loading: true }
    setIsLoading(true)
    
    timerRef.current = window.setTimeout(() => {
      loadSearchResults(0)
    }, 300)

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
      }
    }
  }, [searchQuery, contentType, voteType, sortField, sortOrder, user?.id, activeTab, loadSearchResults])

  const hasResults = searchResults.posts.length > 0 || searchResults.comments.length > 0

  useEffect(() => {
    const sentinel = loadMoreRef.current
    if (!sentinel || !hasResults) return

    const observer = new IntersectionObserver(
      (entries) => {
        const { loading, hasMore: more, activeTab: tab } = stateRef.current
        if (entries[0].isIntersecting && !loading && more && tab === "search") {
          stateRef.current.loading = true
          setLoadingMore(true)
          loadSearchResults(getNextOffset())
        }
      },
      { rootMargin: "200px" }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [loadSearchResults, hasResults])

  useEffect(() => {
    const handleScroll = () => {
      const scrollTop = window.pageYOffset || document.documentElement.scrollTop
      setShowBackToTop(scrollTop > 300)
    }

    window.addEventListener("scroll", handleScroll)
    return () => window.removeEventListener("scroll", handleScroll)
  }, [])

  const clearSearch = () => {
    setSearchQuery("")
    setSearchResults({ posts: [], comments: [] })
    setContentType("all")
    setVoteType(undefined)
    setSortField("createdAt")
    setSortOrder("desc")
  }

  const handleContentTypeChange = (option?: { value: string }) => {
    if (option) {
      setContentType(option.value as ContentType)
    }
  }

  const handleVoteTypeChange = (option?: { value: string }) => {
    if (option) {
      setVoteType(option.value as "up" | "down" | undefined)
    }
  }

  const handleSortChange = (field: SortField) => {
    setSortField(field)
    setSortOrder(prevOrder => (prevOrder === "desc" && field === sortField ? "asc" : "desc"))
  }

  const scrollToTop = () => {
    window.scrollTo({ top: 0, behavior: "smooth" })
  }

  const getFilteredResults = () => {
    let results: Array<any> = []

    if (contentType === "all" || contentType === "posts" || contentType === "voted") {
      results = [...results, ...searchResults.posts.map(post => ({ ...post, type: "post" }))]
    }
    if (contentType === "all" || contentType === "comments") {
      results = [...results, ...searchResults.comments.map(comment => ({ ...comment, type: "comment" }))]
    }

    return results
  }

  if (!user) return null

  const filteredResults = getFilteredResults()

  return (
    <>
      <div className="flex flex-col gap-3 mb-4">
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex-1 min-w-[200px]">
            <Input
              field="search"
              icon={searchQuery ? IconX : IconSearch}
              onIconClick={searchQuery ? clearSearch : undefined}
              value={searchQuery}
              onChange={setSearchQuery}
              placeholder={i18n._("profile.search.placeholder", { message: "Search posts and comments..." })}
            />
          </div>
          <Select
            field="contentType"
            defaultValue={contentType}
            onChange={handleContentTypeChange}
            options={[
              { value: "all", label: i18n._("profile.search.filter.all", { message: "All" }) },
              { value: "posts", label: i18n._("profile.search.filter.posts", { message: "Posts" }) },
              { value: "comments", label: i18n._("profile.search.filter.comments", { message: "Comments" }) },
              { value: "voted", label: i18n._("profile.search.filter.voted", { message: "Voted" }) },
            ]}
          />
          {contentType === "voted" && (
            <Select
              field="voteType"
              defaultValue={voteType}
              onChange={handleVoteTypeChange}
              options={[
                { value: "up", label: i18n._("profile.search.votes.upvotes", { message: "Upvotes" }) },
                { value: "down", label: i18n._("profile.search.votes.downvotes", { message: "Downvotes" }) },
              ]}
            />
          )}
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted"><Trans id="profile.search.sort.label">Sort:</Trans></span>
          <button
            type="button"
            onClick={() => handleSortChange("createdAt")}
            className={`inline-flex items-center gap-1 px-2 py-1 text-xs rounded cursor-pointer transition-colors ${
              sortField === "createdAt" 
                ? "bg-accent-light text-primary font-medium" 
                : "bg-transparent text-muted hover:text-foreground"
            }`}
          >
            <Trans id="profile.search.sort.date">Date</Trans>
            {sortField === "createdAt" && (
              <Icon sprite={IconSort} className={`h-3 transition-transform ${sortOrder === "asc" ? "rotate-180" : ""}`} />
            )}
          </button>
          <button
            type="button"
            onClick={() => handleSortChange("title")}
            className={`inline-flex items-center gap-1 px-2 py-1 text-xs rounded cursor-pointer transition-colors ${
              sortField === "title" 
                ? "bg-accent-light text-primary font-medium" 
                : "bg-transparent text-muted hover:text-foreground"
            }`}
          >
            <Trans id="profile.search.sort.title">Title</Trans>
            {sortField === "title" && (
              <Icon sprite={IconSort} className={`h-3 transition-transform ${sortOrder === "asc" ? "rotate-180" : ""}`} />
            )}
          </button>
        </div>
      </div>

      {filteredResults.length > 0 ? (
        <div className="flex flex-col bg-elevated rounded-card border border-border overflow-hidden">
          {filteredResults.map(item => (
            <a 
              key={`${item.type}-${item.id}`} 
              href={item.type === "comment" ? `/posts/${item.postNumber}#comment-${item.id}` : `/posts/${item.id}`}
              className="flex items-center gap-3 px-4 py-3 border-b border-surface-alt last:border-b-0 hover:bg-tertiary transition-colors no-underline"
            >
              <div className="flex-1 min-w-0">
                {item.type === "post" ? (
                  <div className="text-foreground font-medium text-sm leading-snug truncate">
                    {item.title}
                  </div>
                ) : (
                  <div className="flex flex-col gap-0.5">
                    <div className="text-foreground text-sm leading-snug truncate">
                      {parseMentions(item.content.substring(0, 100))}{item.content.length > 100 ? "..." : ""}
                    </div>
                    <div className="text-xs text-subtle truncate">
                      on "{item.postTitle}"
                    </div>
                  </div>
                )}
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <span className="text-xs text-subtle">
                  {new Date(item.createdAt).toLocaleDateString()}
                </span>
                <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                  item.type === "post" 
                    ? "bg-accent-light text-primary" 
                    : "bg-surface-alt text-muted"
                }`}>
                  {item.type === "post" ? "Post" : "Comment"}
                </span>
              </div>
            </a>
          ))}

          {loadingMore && (
            <div className="flex justify-center p-4">
              <Loader />
            </div>
          )}

          {hasMore && <div ref={loadMoreRef} style={{ height: "1px" }} />}
        </div>
      ) : isLoading ? (
        <div className="flex justify-center p-10">
          <Loader />
        </div>
      ) : (
        <div className="text-center p-10 text-muted">
          <Trans id="profile.search.noresults">No results found</Trans>
        </div>
      )}

      {showBackToTop && (
        <button 
          type="button"
          onClick={scrollToTop} 
          className="fixed bottom-24 right-4 flex items-center gap-1.5 px-3 py-2 rounded-button border border-border shadow-lg z-10 cursor-pointer text-sm bg-elevated text-foreground hover:bg-tertiary hover:border-border-strong transition-colors max-md:bottom-20 max-md:right-3"
        >
          <Icon sprite={IconChevronUp} className="h-4" />
          <Trans id="profile.search.backtotop">Top</Trans>
        </button>
      )}
    </>
  )
}

UserProfileSearchComponent.displayName = "UserProfileSearch"

export const UserProfileSearch = UserProfileSearchComponent
