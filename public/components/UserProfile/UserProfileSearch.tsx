import React, { useState, useEffect, useRef, useCallback } from "react"
import { Input, Select, Button, Icon, Loader, Avatar, UserName, Markdown } from "@fider/components"
import { HStack, VStack } from "@fider/components/layout"
import { useUserProfile } from "./context"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { actions } from "@fider/services"
import { heroiconsSearch as IconSearch, heroiconsX as IconX, heroiconsCalendar as IconCalendar, heroiconsArrowUpDown as IconSort, heroiconsPencilAlt as IconDocument, heroiconsChevronUp as IconChevronUp } from "@fider/icons.generated"

type ContentType = "all" | "posts" | "comments" | "voted"
type SortField = "createdAt" | "title"
type SortOrder = "asc" | "desc"

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
  const containerRef = useRef<HTMLDivElement>(null)

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
      } else {
        setSearchResults(prev => ({
          posts: [...prev.posts, ...result.data.posts],
          comments: [...prev.comments, ...result.data.comments],
        }))
      }
      const hasMorePosts = result.data.posts.length === options.limit
      const hasMoreComments = result.data.comments.length === options.limit
      setHasMore(hasMorePosts || hasMoreComments)
    }
    setIsLoading(false)
    setLoadingMore(false)
  }, [user, contentType, voteType, sortField, sortOrder, searchQuery])

  const loadMoreResults = useCallback(() => {
    if (loadingMore || !hasMore) return
    setLoadingMore(true)
    const nextOffset = searchResults.posts.length + searchResults.comments.length
    loadSearchResults(nextOffset)
  }, [loadingMore, hasMore, searchResults, loadSearchResults])

  useEffect(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }

    setHasMore(true)

    if (activeTab !== "search") return

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

  useEffect(() => {
    const handleScroll = () => {
      const scrollTop = window.pageYOffset || document.documentElement.scrollTop
      setShowBackToTop(scrollTop > 300)

      if (
        containerRef.current &&
        activeTab === "search" &&
        !isLoading &&
        !loadingMore &&
        hasMore
      ) {
        const containerBottom = containerRef.current.getBoundingClientRect().bottom
        const windowHeight = window.innerHeight

        if (containerBottom <= windowHeight + 200) {
          loadMoreResults()
        }
      }
    }

    window.addEventListener("scroll", handleScroll)
    return () => window.removeEventListener("scroll", handleScroll)
  }, [activeTab, isLoading, loadingMore, hasMore, loadMoreResults])

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

  const userForComponents = {
    ...user,
    role: user.role as any,
    status: user.status as any,
  }

  const filteredResults = getFilteredResults()

  return (
    <>
      <div className="c-user-profile__search-controls">
        <div className="c-user-profile__search-input">
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

      {isLoading && searchResults.posts.length === 0 && searchResults.comments.length === 0 ? (
        <div className="c-user-profile__loading">
          <Loader />
        </div>
      ) : searchResults.posts.length > 0 || searchResults.comments.length > 0 ? (
        <div className="c-user-profile__search-results" ref={containerRef}>
          <div className="c-user-profile__search-header">
            <div className="c-user-profile__search-sort">
              <Button
                variant="tertiary"
                onClick={() => handleSortChange("createdAt")}
                className={sortField === "createdAt" ? "active" : ""}
              >
                <HStack spacing={2}>
                  <Icon sprite={IconCalendar} className="h-4" />
                  <span><Trans id="profile.search.sort.date">Date</Trans></span>
                  {sortField === "createdAt" && (
                    <Icon sprite={IconSort} className={`h-4 transform ${sortOrder === "asc" ? "rotate-180" : ""}`} />
                  )}
                </HStack>
              </Button>
              <Button
                variant="tertiary"
                onClick={() => handleSortChange("title")}
                className={sortField === "title" ? "active" : ""}
              >
                <HStack spacing={2}>
                  <Icon sprite={IconDocument} className="h-4" />
                  <span><Trans id="profile.search.sort.title">Title</Trans></span>
                  {sortField === "title" && (
                    <Icon sprite={IconSort} className={`h-4 transform ${sortOrder === "asc" ? "rotate-180" : ""}`} />
                  )}
                </HStack>
              </Button>
            </div>
          </div>

          <VStack spacing={4} divide>
            {filteredResults.map(item => (
              <div key={`${item.type}-${item.id}`} className="c-user-profile__search-item" data-type={item.type}>
                {item.type === "comment" ? (
                  <>
                    <div className="flex-items-baseline flex flex-x flex--spacing-2">
                      <div className="pt-4">
                        <a href={`/profile/${user.id}`}>
                          <Avatar user={userForComponents} clickable={false} size="small" />
                        </a>
                      </div>
                      <div className="flex-grow rounded-md p-2">
                        <div className="mb-1">
                          <div className="flex flex-x flex--spacing-2 justify-between flex-items-center">
                            <div className="flex flex-x flex--spacing-2 flex-items-center">
                              <UserName user={userForComponents} />
                              <div className="text-xs">
                                · <span className="date">{new Date(item.createdAt).toLocaleDateString()}</span>
                              </div>
                            </div>
                          </div>
                        </div>
                        <div>
                          <div className="c-markdown">
                            <Markdown text={item.content} style="full" />
                          </div>
                        </div>
                      </div>
                    </div>
                    <div className="c-user-profile__comment-actions">
                      <a href={`/posts/${item.postNumber}#comment-${item.id}`} className="comment-link">
                        <Trans id="profile.search.comment.view">View in context</Trans>
                      </a>
                    </div>
                  </>
                ) : (
                  <div className="c-user-profile__post-card">
                    <div className="c-user-profile__post-header">
                      <div className="c-user-profile__post-meta">
                        <div className="flex flex-x flex--spacing-2 flex-items-center">
                          <a href={`/profile/${user.id}`}>
                            <Avatar user={userForComponents} size="small" clickable={false} />
                          </a>
                          <UserName user={userForComponents} />
                          <div className="text-xs">
                            · <span className="date">{new Date(item.createdAt).toLocaleDateString()}</span>
                          </div>
                        </div>
                      </div>
                    </div>
                    <div className="c-user-profile__post-title">
                      <a href={`/posts/${item.id}`}>{item.title}</a>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </VStack>

          {loadingMore && (
            <div className="c-user-profile__loading-more">
              <Loader />
            </div>
          )}
        </div>
      ) : searchQuery ? (
        <div className="c-user-profile__no-results">
          <Trans id="profile.search.noresults">No results found</Trans>
        </div>
      ) : null}

      {showBackToTop && (
        <button onClick={scrollToTop} className="c-user-profile__back-to-top c-button--primary">
          <Icon sprite={IconChevronUp} className="h-4" />
          <Trans id="profile.search.backtotop">Back to Top</Trans>
        </button>
      )}
    </>
  )
}

UserProfileSearchComponent.displayName = "UserProfileSearch"

export const UserProfileSearch = UserProfileSearchComponent

