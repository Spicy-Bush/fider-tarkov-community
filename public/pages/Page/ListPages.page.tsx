import React, { useState, useEffect, useRef, useCallback } from "react"
import { Page, PageTopic, PageTag } from "@fider/models"
import { Input } from "@fider/components"
import { VStack, HStack } from "@fider/components/layout"
import { heroiconsSearch as IconSearch } from "@fider/icons.generated"
import { searchPages } from "@fider/services/pages"
import { PageFilterPanel, PageFilterState } from "./components/PageFilterPanel"

interface ListPagesProps {
  pages: Page[]
  topics: PageTopic[]
  tags: PageTag[]
  totalCount: number
  totalPages: number
  page: number
  query?: string
  topicFilter?: string
  tagFilter?: string
}

const ListPages = ({ pages: initialPages, topics, tags, totalCount: initialTotal, totalPages: initialTotalPages, page: initialPage, query, topicFilter, tagFilter }: ListPagesProps) => {
  const [pages, setPages] = useState<Page[]>(initialPages)
  const [totalCount, setTotalCount] = useState(initialTotal)
  const [totalPages, setTotalPages] = useState(initialTotalPages)
  const [currentPage, setCurrentPage] = useState(initialPage)
  const [searchQuery, setSearchQuery] = useState(query || "")
  const [isLoading, setIsLoading] = useState(false)
  const timerRef = useRef<number>()

  const [filter, setFilter] = useState<PageFilterState>({
    tags: tagFilter ? tagFilter.split(",") : [],
    topic: topicFilter || "",
  })

  const doSearch = useCallback(async (q: string, filterState: PageFilterState, page: number = 1) => {
    setIsLoading(true)
    const result = await searchPages({
      q: q || undefined,
      topics: filterState.topic || undefined,
      tags: filterState.tags.length > 0 ? filterState.tags : undefined,
      page,
    })
    if (result.ok) {
      setPages(result.data.pages)
      setTotalCount(result.data.totalCount)
      setTotalPages(result.data.totalPages)
      setCurrentPage(result.data.page)
      
      const params = new URLSearchParams()
      if (q) params.set("q", q)
      if (filterState.topic) params.set("topics", filterState.topic)
      if (filterState.tags.length > 0) params.set("tags", filterState.tags.join(","))
      if (page > 1) params.set("page", page.toString())
      const newUrl = `/pages${params.toString() ? `?${params.toString()}` : ""}`
      window.history.replaceState({}, "", newUrl)
    }
    setIsLoading(false)
  }, [])

  const handleSearchChange = (value: string) => {
    setSearchQuery(value)
    if (timerRef.current) clearTimeout(timerRef.current)
    timerRef.current = window.setTimeout(() => {
      doSearch(value, filter)
    }, 400)
  }

  const handleFilterChange = (newFilter: PageFilterState) => {
    setFilter(newFilter)
    doSearch(searchQuery, newFilter)
  }

  const handlePageChange = (page: number) => {
    doSearch(searchQuery, filter, page)
  }

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, [])

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    return date.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })
  }

  const activeFilterSummary = () => {
    const parts: string[] = []
    if (filter.topic) {
      const topic = topics.find(t => t.slug === filter.topic)
      if (topic) parts.push(topic.name)
    }
    if (filter.tags.length > 0) {
      const tagNames = filter.tags.map(slug => {
        const tag = tags.find(t => t.slug === slug)
        return tag?.name || slug
      })
      parts.push(...tagNames)
    }
    return parts
  }

  const filterSummary = activeFilterSummary()

  return (
    <div className="page container max-w-4xl mx-auto">
      <div className="mb-8">
        <h1 className="text-display mb-2">Pages</h1>
        <p className="text-muted">Browse our collection of guides, documentation, and resources.</p>
      </div>

      <div className="mb-6">
        <HStack spacing={4} align="center" className="flex-wrap gap-y-3">
          <div className="flex-1 min-w-[200px]">
            <Input
              field="search"
              placeholder="Search pages..."
              value={searchQuery}
              onChange={handleSearchChange}
              icon={IconSearch}
              noMargin
            />
          </div>
          
          {(topics.length > 0 || tags.length > 0) && (
            <PageFilterPanel
              topics={topics}
              tags={tags}
              activeFilter={filter}
              filtersChanged={handleFilterChange}
            />
          )}
        </HStack>

        {filterSummary.length > 0 && (
          <div className="mt-3 flex flex-wrap gap-2">
            {filterSummary.map((item, idx) => (
              <span key={idx} className="px-2 py-1 text-xs rounded-badge bg-primary/10 text-primary border border-primary/20">
                {item}
              </span>
            ))}
            <button 
              onClick={() => handleFilterChange({ tags: [], topic: "" })}
              className="px-2 py-1 text-xs rounded-badge bg-tertiary text-muted hover:text-foreground cursor-pointer"
            >
              Clear all
            </button>
          </div>
        )}
      </div>

      {totalCount > 0 && (
        <p className="text-sm text-muted mb-6">
          {totalCount} page{totalCount !== 1 ? "s" : ""} found
          {isLoading && <span className="ml-2 text-primary">Loading...</span>}
        </p>
      )}

      <VStack spacing={0} className={`divide-y divide-border ${isLoading ? "opacity-50" : ""}`}>
        {pages.map((page) => (
          <a
            key={page.id}
            href={`/pages/${page.slug}`}
            className="group flex gap-6 py-6 first:pt-0 last:pb-0 hover:bg-tertiary/30 -mx-4 px-4 rounded-card transition-colors"
          >
            {page.bannerImageBKey && (
              <div className="hidden sm:flex shrink-0 w-20 h-14 items-center justify-center">
                <img
                  src={`/static/images/${page.bannerImageBKey}`}
                  alt={page.title}
                  className="w-20 h-14 object-cover rounded"
                />
              </div>
            )}
            
            <div className="flex-1 min-w-0">
              <div className="flex items-start justify-between gap-4 mb-2">
                <h2 className="text-lg font-semibold text-foreground group-hover:text-primary transition-colors line-clamp-2">
                  {page.title}
                </h2>
                <span className="text-xs text-subtle whitespace-nowrap shrink-0">
                  {formatDate(page.updatedAt)}
                </span>
              </div>
              
              {page.excerpt && (
                <p className="text-muted text-sm mb-3 line-clamp-2">{page.excerpt}</p>
              )}
              
              <HStack spacing={4} className="text-xs text-subtle">
                <span>{Math.max(1, Math.ceil(page.content.split(/\s+/).length / 250))} min read</span>
                
                {page.topics && page.topics.length > 0 && (
                  <HStack spacing={2}>
                    {page.topics.slice(0, 3).map((topic: any) => (
                      <span
                        key={topic.id}
                        className="px-2 py-0.5 text-xs rounded-full bg-tertiary text-muted"
                      >
                        {topic.name}
                      </span>
                    ))}
                    {page.topics.length > 3 && (
                      <span className="text-muted">+{page.topics.length - 3}</span>
                    )}
                  </HStack>
                )}
              </HStack>
            </div>
          </a>
        ))}
      </VStack>

      {pages.length === 0 && (
        <div className="text-center py-12">
          <p className="text-muted">No pages found.</p>
        </div>
      )}

      {totalPages > 1 && (
        <HStack spacing={2} justify="center" className="mt-10 pt-6 border-t border-border">
          {currentPage > 1 && (
            <button
              onClick={() => handlePageChange(currentPage - 1)}
              className="px-4 py-2 rounded-button bg-tertiary hover:bg-tertiary-hover text-foreground text-sm"
              disabled={isLoading}
            >
              Previous
            </button>
          )}
          
          {Array.from({ length: totalPages }, (_, i) => i + 1)
            .filter(num => num === 1 || num === totalPages || Math.abs(num - currentPage) <= 1)
            .map((pageNum, idx, arr) => (
              <React.Fragment key={pageNum}>
                {idx > 0 && arr[idx - 1] !== pageNum - 1 && (
                  <span className="px-2 text-muted">...</span>
                )}
                <button
                  onClick={() => handlePageChange(pageNum)}
                  className={`px-4 py-2 rounded-button text-sm ${
                    pageNum === currentPage
                      ? "bg-primary text-primary-foreground"
                      : "bg-tertiary hover:bg-tertiary-hover text-foreground"
                  }`}
                  disabled={isLoading}
                >
                  {pageNum}
                </button>
              </React.Fragment>
            ))}
          
          {currentPage < totalPages && (
            <button
              onClick={() => handlePageChange(currentPage + 1)}
              className="px-4 py-2 rounded-button bg-tertiary hover:bg-tertiary-hover text-foreground text-sm"
              disabled={isLoading}
            >
              Next
            </button>
          )}
        </HStack>
      )}
    </div>
  )
}

export default ListPages
