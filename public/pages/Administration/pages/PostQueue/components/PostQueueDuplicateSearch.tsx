import React, { useState, useEffect, useCallback, useRef } from "react"
import {
  Button,
  Icon,
  Input,
  Loader,
  Avatar,
  Moment,
  Markdown,
  ImageGallery,
  ShowTag,
} from "@fider/components"
import { HStack, VStack } from "@fider/components/layout"
import { Post, Tag, Comment, PostStatus } from "@fider/models"
import { actions, Fider, classSet } from "@fider/services"
import {
  heroiconsSearch as IconSearch,
  heroiconsX as IconX,
  heroiconsChevronDown as IconChevronDown,
  heroiconsChevronUp as IconChevronUp,
  heroiconsArrowLeft as IconArrowLeft,
} from "@fider/icons.generated"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"

import "./PostQueueDuplicateSearch.scss"

interface PostQueueDuplicateSearchProps {
  excludePostNumber: number
  tags: Tag[]
  onSelect: (postNumber: number) => void
  onCancel: () => void
}

interface ExpandedPostData {
  post: Post
  comments: Comment[]
  attachments: string[]
}

export const PostQueueDuplicateSearch: React.FC<PostQueueDuplicateSearchProps> = ({
  excludePostNumber,
  tags,
  onSelect,
  onCancel,
}) => {
  const [query, setQuery] = useState("")
  const [selectedTags, setSelectedTags] = useState<string[]>([])
  const [posts, setPosts] = useState<Post[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [expandedPostId, setExpandedPostId] = useState<number | null>(null)
  const [expandedData, setExpandedData] = useState<ExpandedPostData | null>(null)
  const [isLoadingExpanded, setIsLoadingExpanded] = useState(false)
  const [showTagFilter, setShowTagFilter] = useState(false)
  const [showComments, setShowComments] = useState(false)
  const resultRefs = useRef<Map<number, HTMLDivElement>>(new Map())

  const searchPosts = useCallback(async () => {
    setIsLoading(true)
    const result = await actions.searchPosts({
      query,
      tags: selectedTags,
      statuses: ["open", "planned", "started", "completed", "declined"],
      limit: 20,
      view: "newest",
    })
    if (result.ok) {
      const filteredPosts = (result.data || []).filter(
        (p) => p.number !== excludePostNumber
      )
      setPosts(filteredPosts)
    }
    setIsLoading(false)
  }, [query, selectedTags, excludePostNumber])

  useEffect(() => {
    const timer = setTimeout(() => {
      searchPosts()
    }, 300)
    return () => clearTimeout(timer)
  }, [searchPosts])

  const handleExpand = async (post: Post) => {
    const ref = resultRefs.current.get(post.id)
    
    if (expandedPostId === post.id) {
      setExpandedPostId(null)
      setExpandedData(null)
      setTimeout(() => {
        ref?.scrollIntoView({ block: "nearest", behavior: "instant" })
      }, 0)
      return
    }

    setExpandedPostId(post.id)
    setIsLoadingExpanded(true)
    setShowComments(false)

    const [commentsResult, attachmentsResult] = await Promise.all([
      actions.getAllComments(post.number),
      actions.getPostAttachments(post.number),
    ])

    setExpandedData({
      post,
      comments: commentsResult.ok ? commentsResult.data || [] : [],
      attachments: attachmentsResult.ok ? attachmentsResult.data || [] : [],
    })
    setIsLoadingExpanded(false)
    
    setTimeout(() => {
      ref?.scrollIntoView({ block: "nearest", behavior: "instant" })
    }, 0)
  }

  const toggleTag = (slug: string) => {
    setSelectedTags((prev) =>
      prev.includes(slug) ? prev.filter((t) => t !== slug) : [...prev, slug]
    )
  }

  return (
    <div className="c-duplicate-search">
      <div className="c-duplicate-search__header">
        <Button variant="tertiary" size="small" onClick={onCancel}>
          <Icon sprite={IconArrowLeft} className="h-4" />
          <span>Back</span>
        </Button>
        <h3 className="c-duplicate-search__title">Find Original Post</h3>
      </div>

      <div className="c-duplicate-search__filters">
        <div className="c-duplicate-search__search">
          <Input
            field="query"
            icon={query ? IconX : IconSearch}
            onIconClick={query ? () => setQuery("") : undefined}
            placeholder={i18n._("duplicate.search.placeholder", { message: "Search posts..." })}
            value={query}
            onChange={setQuery}
          />
        </div>
        <Button
          variant="secondary"
          size="small"
          onClick={() => setShowTagFilter(!showTagFilter)}
          className={classSet({
            "c-duplicate-search__tag-btn": true,
            "c-duplicate-search__tag-btn--active": selectedTags.length > 0,
          })}
        >
          Tags {selectedTags.length > 0 && `(${selectedTags.length})`}
          <Icon sprite={showTagFilter ? IconChevronUp : IconChevronDown} className="h-4" />
        </Button>
      </div>

      {showTagFilter && (
        <div className="c-duplicate-search__tag-filter">
          <div className="c-duplicate-search__tag-grid">
            {tags.map((tag) => (
              <div
                key={tag.id}
                className="c-duplicate-search__tag-item"
                onClick={() => toggleTag(tag.slug)}
              >
                <ShowTag tag={tag} selectable selected={selectedTags.includes(tag.slug)} />
              </div>
            ))}
          </div>
          {selectedTags.length > 0 && (
            <Button variant="tertiary" size="small" onClick={() => setSelectedTags([])}>
              Clear all
            </Button>
          )}
        </div>
      )}

      <div className="c-duplicate-search__results">
        {isLoading ? (
          <div className="py-8 text-center">
            <Loader />
          </div>
        ) : posts.length === 0 ? (
          <div className="py-8 text-center text-muted">
            <Trans id="duplicate.search.noresults">No matching posts found.</Trans>
          </div>
        ) : (
          <VStack spacing={0}>
            {posts.map((post) => {
              const isExpanded = expandedPostId === post.id
              const postTags = tags.filter((t) => post.tags.includes(t.slug))

              return (
                <div
                  key={post.id}
                  className="c-duplicate-search__result"
                  ref={(el) => {
                    if (el) resultRefs.current.set(post.id, el)
                  }}
                >
                  <div className="c-duplicate-search__result-header" onClick={() => handleExpand(post)}>
                    <div className="c-duplicate-search__result-info">
                      <div className="c-duplicate-search__result-meta">
                        <span className="c-duplicate-search__result-number">#{post.number}</span>
                        <span className={classSet({
                          "c-duplicate-search__result-status": true,
                          "bg-gray-200 text-gray-700": post.status === "open",
                          "bg-blue-100 text-blue-700": post.status === "planned",
                          "bg-green-100 text-green-700": post.status === "started",
                          "bg-green-300 text-green-800": post.status === "completed",
                          "bg-red-100 text-red-700": post.status === "declined",
                        })}>
                          {PostStatus.Get(post.status).title}
                        </span>
                        <span className={classSet({
                          "c-duplicate-search__result-votes": true,
                          "c-duplicate-search__result-votes--positive": (post.upvotes || 0) - (post.downvotes || 0) > 0,
                          "c-duplicate-search__result-votes--negative": (post.upvotes || 0) - (post.downvotes || 0) < 0,
                        })}>
                          {(post.upvotes || 0) - (post.downvotes || 0)} votes
                        </span>
                        <span className="c-duplicate-search__result-comments">
                          {post.commentsCount} comments
                        </span>
                      </div>
                      <div className="c-duplicate-search__result-title">{post.title}</div>
                      {postTags.length > 0 && (
                        <HStack spacing={1} className="c-duplicate-search__result-tags">
                          {postTags.slice(0, 3).map((tag) => (
                            <ShowTag key={tag.id} tag={tag} />
                          ))}
                          {postTags.length > 3 && (
                            <span className="text-xs text-muted">+{postTags.length - 3}</span>
                          )}
                        </HStack>
                      )}
                    </div>
                    <HStack spacing={2}>
                      <Button
                        variant="primary"
                        size="small"
                        onClick={() => onSelect(post.number)}
                      >
                        Select
                      </Button>
                      <Icon
                        sprite={isExpanded ? IconChevronUp : IconChevronDown}
                        className="h-5 text-muted"
                      />
                    </HStack>
                  </div>

                  {isExpanded && (
                    <div className="c-duplicate-search__result-expanded">
                      {isLoadingExpanded ? (
                        <div className="py-4 text-center">
                          <Loader />
                        </div>
                      ) : expandedData ? (
                        <>
                          <div className="c-duplicate-search__content">
                            <span className="text-xs text-muted uppercase mb-2 block">Description</span>
                            {expandedData.post.description ? (
                              <Markdown text={expandedData.post.description} style="full" />
                            ) : (
                              <em className="text-muted">No description</em>
                            )}
                            {expandedData.attachments.length > 0 && (
                              <div className="mt-2">
                                <ImageGallery bkeys={expandedData.attachments} />
                              </div>
                            )}
                          </div>

                          {expandedData.comments.length > 0 && (
                            <div className="c-duplicate-search__comments">
                              <button
                                className="c-duplicate-search__comments-toggle"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  setShowComments(!showComments)
                                }}
                              >
                                <span className="text-xs text-muted uppercase">
                                  Comments ({expandedData.comments.length})
                                </span>
                                <Icon
                                  sprite={showComments ? IconChevronUp : IconChevronDown}
                                  className="h-4 text-muted"
                                />
                              </button>
                              {showComments && (
                                <VStack spacing={2} className="mt-2">
                                  {expandedData.comments.slice(0, 5).map((comment) => (
                                    <div key={comment.id} className="c-duplicate-search__comment">
                                      <HStack spacing={2} className="mb-1">
                                        <Avatar user={comment.user} clickable={false} />
                                        <span className="text-sm font-medium">
                                          {comment.user.name}
                                        </span>
                                        <Moment
                                          locale={Fider.currentLocale}
                                          date={comment.createdAt}
                                        />
                                      </HStack>
                                      <Markdown text={comment.content} style="full" />
                                    </div>
                                  ))}
                                  {expandedData.comments.length > 5 && (
                                    <span className="text-sm text-muted">
                                      +{expandedData.comments.length - 5} more comments
                                    </span>
                                  )}
                                </VStack>
                              )}
                            </div>
                          )}
                        </>
                      ) : null}
                    </div>
                  )}
                </div>
              )
            })}
          </VStack>
        )}
      </div>
    </div>
  )
}

