import React, { useState, useCallback, useEffect } from "react"
import { Post, Tag, PostStatus } from "@fider/models"
import { actions } from "@fider/services"
import { Button, Icon, Moment, ShowTag, Checkbox, Markdown, ImageGallery, Loader } from "@fider/components"
import { PageConfig } from "@fider/components/layouts"
import { VStack, HStack } from "@fider/components/layout"
import { Trans } from "@lingui/react/macro"
import { useFider } from "@fider/hooks"
import { 
  heroiconsRefresh as IconRefresh, 
  heroiconsFilter as IconFilter,
  heroiconsChevronDown as IconChevronDown,
  heroiconsChevronUp as IconChevronUp
} from "@fider/icons.generated"

import "./ManageArchive.page.scss"

export const pageConfig: PageConfig = {
  title: "Archive Posts",
  subtitle: "Archive low-engagement or stale posts",
  sidebarItem: "archive",
  layoutVariant: "default",
}

interface ManageArchivePageProps {
  tags: Tag[]
}

interface ExpandablePostItemProps {
  post: Post
  tags: Tag[]
  isSelected: boolean
  onSelect: () => void
  locale: string
}

const ExpandablePostItem: React.FC<ExpandablePostItemProps> = ({ post, tags, isSelected, onSelect, locale }) => {
  const [isExpanded, setIsExpanded] = useState(false)
  const [attachments, setAttachments] = useState<string[]>([])
  const [isLoadingAttachments, setIsLoadingAttachments] = useState(false)

  const handleExpand = async () => {
    const willExpand = !isExpanded
    setIsExpanded(willExpand)
    
    if (willExpand && attachments.length === 0) {
      setIsLoadingAttachments(true)
      const result = await actions.getPostAttachments(post.number)
      if (result.ok && result.data) {
        setAttachments(result.data)
      }
      setIsLoadingAttachments(false)
    }
  }

  return (
    <div className={`c-archive-page__item ${isSelected ? "c-archive-page__item--selected" : ""}`}>
      <div className="c-archive-page__item-row">
        <Checkbox
          field={`select-${post.id}`}
          checked={isSelected}
          onChange={onSelect}
          expandedHitbox
        />
        <div className="c-archive-page__item-content" onClick={handleExpand}>
          <div className="c-archive-page__item-header">
            <a
              href={`/posts/${post.number}/${post.slug}`}
              target="_blank"
              rel="noopener noreferrer"
              className="c-archive-page__item-title"
              onClick={(e) => e.stopPropagation()}
            >
              {post.title}
            </a>
            <Icon 
              sprite={isExpanded ? IconChevronUp : IconChevronDown} 
              className="c-archive-page__item-expand h-4"
            />
          </div>
          <div className="c-archive-page__item-meta">
            <span className="c-archive-page__item-number">#{post.number}</span>
            <span className="c-archive-page__item-stat">{post.votesCount} votes</span>
            <span className="c-archive-page__item-stat">{post.commentsCount} comments</span>
          </div>
          <div className="c-archive-page__item-dates">
            <span>
              Created: <Moment locale={locale} date={post.createdAt} />
            </span>
            <span>
              Last activity: <Moment locale={locale} date={post.lastActivityAt} />
            </span>
          </div>
          {post.tags.length > 0 && (
            <div className="c-archive-page__item-tags">
              {post.tags.map((slug) => {
                const tag = tags.find((t) => t.slug === slug)
                return tag ? <ShowTag key={tag.id} tag={tag} size="mini" /> : null
              })}
            </div>
          )}
        </div>
        <div className="c-archive-page__item-status">
          {PostStatus.Get(post.status).title}
        </div>
      </div>
      {isExpanded && (
        <div className="c-archive-page__item-description">
          {isLoadingAttachments ? (
            <div className="c-archive-page__item-loading">
              <Loader />
            </div>
          ) : (
            <>
              {post.description ? (
                <Markdown text={post.description} style="full" />
              ) : (
                <span className="text-muted">No description</span>
              )}
              {attachments.length > 0 && (
                <div className="c-archive-page__item-attachments">
                  <ImageGallery bkeys={attachments} />
                </div>
              )}
            </>
          )}
        </div>
      )}
    </div>
  )
}

const ManageArchivePage: React.FC<ManageArchivePageProps> = (props) => {
  const fider = useFider()
  const [posts, setPosts] = useState<Post[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [isLoading, setIsLoading] = useState(false)
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [showFilters, setShowFilters] = useState(false)

  const [maxVotes, setMaxVotes] = useState<string>("")
  const [maxComments, setMaxComments] = useState<string>("")
  const [inactiveDays, setInactiveDays] = useState<string>("")
  const [createdDaysAgo, setCreatedDaysAgo] = useState<string>("")
  const [filterStatus, setFilterStatus] = useState<string>("open")
  const [filterTags, setFilterTags] = useState<string[]>([])

  const perPage = 50

  const loadPosts = useCallback(async () => {
    setIsLoading(true)
    setSelectedIds(new Set())

    const params: actions.GetArchivablePostsParams = {
      page,
      perPage,
    }

    if (maxVotes) params.maxVotes = parseInt(maxVotes, 10)
    if (maxComments) params.maxComments = parseInt(maxComments, 10)
    if (inactiveDays) {
      const date = new Date()
      date.setDate(date.getDate() - parseInt(inactiveDays, 10))
      params.inactiveSince = date.toISOString()
    }
    if (createdDaysAgo) {
      const date = new Date()
      date.setDate(date.getDate() - parseInt(createdDaysAgo, 10))
      params.createdBefore = date.toISOString()
    }
    if (filterStatus) {
      params.statuses = [filterStatus]
    }
    if (filterTags.length > 0) {
      params.tags = filterTags
    }

    const result = await actions.getArchivablePosts(params)
    if (result.ok && result.data) {
      setPosts(result.data.posts)
      setTotal(result.data.total)
    }
    setIsLoading(false)
  }, [page, maxVotes, maxComments, inactiveDays, createdDaysAgo, filterStatus, filterTags])

  useEffect(() => {
    loadPosts()
  }, [loadPosts])

  const handleSelectAll = () => {
    if (selectedIds.size === posts.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(posts.map((p) => p.id)))
    }
  }

  const handleSelectPost = (postId: number) => {
    const newSelected = new Set(selectedIds)
    if (newSelected.has(postId)) {
      newSelected.delete(postId)
    } else {
      newSelected.add(postId)
    }
    setSelectedIds(newSelected)
  }

  const handleArchiveSelected = async () => {
    if (selectedIds.size === 0) return

    const confirmed = window.confirm(
      `Are you sure you want to archive ${selectedIds.size} post(s)? They can be unarchived later.`
    )
    if (!confirmed) return

    const result = await actions.bulkArchivePosts(Array.from(selectedIds))
    if (result.ok) {
      loadPosts()
    }
  }

  const handleApplyPreset = (preset: "stale" | "lowEngagement") => {
    if (preset === "stale") {
      setCreatedDaysAgo("180")
      setInactiveDays("90")
      setMaxVotes("5")
      setMaxComments("")
      setFilterStatus("open")
      setFilterTags([])
    } else if (preset === "lowEngagement") {
      setCreatedDaysAgo("30")
      setInactiveDays("")
      setMaxVotes("3")
      setMaxComments("0")
      setFilterStatus("open")
      setFilterTags([])
    }
    setPage(1)
  }

  const handleClearFilters = () => {
    setMaxVotes("")
    setMaxComments("")
    setInactiveDays("")
    setCreatedDaysAgo("")
    setFilterStatus("open")
    setFilterTags([])
    setPage(1)
  }

  const handleTagToggle = (slug: string) => {
    setFilterTags((prev) =>
      prev.includes(slug) ? prev.filter((t) => t !== slug) : [...prev, slug]
    )
  }

  const totalPages = Math.ceil(total / perPage)

  return (
    <div className="c-archive-page">
      <div className="c-archive-page__container">
        <VStack spacing={4}>
          <HStack justify="between" className="c-archive-page__header">
            <div className="c-archive-page__count">
              <span className="c-archive-page__count-number">{total}</span> post(s) found
            </div>
            <HStack spacing={2}>
              <Button size="small" variant="secondary" onClick={() => setShowFilters(!showFilters)}>
                <Icon sprite={IconFilter} className="h-4" />
                Filters
              </Button>
              <Button size="small" variant="secondary" onClick={loadPosts}>
                <Icon sprite={IconRefresh} className="h-4" />
              </Button>
            </HStack>
          </HStack>

          {showFilters && (
            <div className="c-archive-page__filters">
              <VStack spacing={4}>
                <div className="c-archive-page__filters-grid">
                  <div className="c-archive-page__filter-group">
                    <label>Max Votes</label>
                    <input
                      type="number"
                      value={maxVotes}
                      onChange={(e) => setMaxVotes(e.target.value)}
                      placeholder="e.g. 5"
                      className="c-archive-page__input"
                    />
                  </div>
                  <div className="c-archive-page__filter-group">
                    <label>Max Comments</label>
                    <input
                      type="number"
                      value={maxComments}
                      onChange={(e) => setMaxComments(e.target.value)}
                      placeholder="e.g. 0"
                      className="c-archive-page__input"
                    />
                  </div>
                  <div className="c-archive-page__filter-group">
                    <label>Inactive (days)</label>
                    <input
                      type="number"
                      value={inactiveDays}
                      onChange={(e) => setInactiveDays(e.target.value)}
                      placeholder="e.g. 90"
                      className="c-archive-page__input"
                    />
                  </div>
                  <div className="c-archive-page__filter-group">
                    <label>Created (days ago)</label>
                    <input
                      type="number"
                      value={createdDaysAgo}
                      onChange={(e) => setCreatedDaysAgo(e.target.value)}
                      placeholder="e.g. 180"
                      className="c-archive-page__input"
                    />
                  </div>
                  <div className="c-archive-page__filter-group">
                    <label>Status</label>
                    <select
                      value={filterStatus}
                      onChange={(e) => setFilterStatus(e.target.value)}
                      className="c-archive-page__select"
                    >
                      <option value="">All</option>
                      <option value="open">Open</option>
                      <option value="planned">Planned</option>
                      <option value="started">Started</option>
                      <option value="completed">Completed</option>
                      <option value="declined">Declined</option>
                    </select>
                  </div>
                </div>
                {props.tags.length > 0 && (
                  <div className="c-archive-page__filter-group c-archive-page__filter-group--tags">
                    <label>Tags {filterTags.length > 0 && `(${filterTags.length} selected)`}</label>
                    <div className="c-archive-page__tags-list">
                      {props.tags.map((tag) => (
                        <div
                          key={tag.id}
                          className="c-archive-page__tag-item"
                          onClick={() => handleTagToggle(tag.slug)}
                        >
                          <ShowTag tag={tag} selectable selected={filterTags.includes(tag.slug)} />
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                <HStack spacing={2} className="c-archive-page__filter-actions">
                  <Button size="small" variant="secondary" onClick={() => handleApplyPreset("stale")}>
                    Preset: Stale
                  </Button>
                  <Button size="small" variant="secondary" onClick={() => handleApplyPreset("lowEngagement")}>
                    Preset: Low Engagement
                  </Button>
                  <Button size="small" variant="tertiary" onClick={handleClearFilters}>
                    Clear
                  </Button>
                  <Button size="small" variant="primary" onClick={() => { setPage(1); loadPosts(); }}>
                    Apply
                  </Button>
                </HStack>
              </VStack>
            </div>
          )}

          <HStack justify="between" className="c-archive-page__actions">
            <Checkbox
              field="selectAll"
              checked={selectedIds.size > 0 && selectedIds.size === posts.length}
              onChange={handleSelectAll}
            >
              Select All ({selectedIds.size} selected)
            </Checkbox>
            <Button
              size="small"
              variant="danger"
              onClick={handleArchiveSelected}
              disabled={selectedIds.size === 0}
            >
              Archive Selected ({selectedIds.size})
            </Button>
          </HStack>

          {isLoading ? (
            <div className="c-archive-page__loading">Loading...</div>
          ) : posts.length === 0 ? (
            <div className="c-archive-page__empty">
              <Trans id="admin.archive.noposts">No posts match the current filters</Trans>
            </div>
          ) : (
            <div className="c-archive-page__list">
              {posts.map((post) => (
                <ExpandablePostItem
                  key={post.id}
                  post={post}
                  tags={props.tags}
                  isSelected={selectedIds.has(post.id)}
                  onSelect={() => handleSelectPost(post.id)}
                  locale={fider.currentLocale}
                />
              ))}
            </div>
          )}

          {totalPages > 1 && (
            <HStack justify="center" spacing={2} className="c-archive-page__pagination">
              <Button
                size="small"
                variant="secondary"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
              >
                Previous
              </Button>
              <span className="c-archive-page__pagination-text">
                Page {page} of {totalPages}
              </span>
              <Button
                size="small"
                variant="secondary"
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
              >
                Next
              </Button>
            </HStack>
          )}
        </VStack>
      </div>
    </div>
  )
}

export default ManageArchivePage
