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
    <div className="h-full flex flex-col bg-elevated">
      <div className="flex items-center gap-3 p-4 border-b border-surface-alt bg-tertiary sticky top-0 z-10">
        <Button variant="tertiary" size="small" onClick={onCancel}>
          <Icon sprite={IconArrowLeft} className="h-4" />
          <span>Back</span>
        </Button>
        <h3 className="text-lg font-semibold m-0">Find Original Post</h3>
      </div>

      <div className="flex gap-2 p-3 px-4 border-b border-surface-alt">
        <div className="flex-1">
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
            "flex items-center gap-1": true,
            "bg-info-medium text-primary": selectedTags.length > 0,
          })}
        >
          Tags {selectedTags.length > 0 && `(${selectedTags.length})`}
          <Icon sprite={showTagFilter ? IconChevronUp : IconChevronDown} className="h-4" />
        </Button>
      </div>

      {showTagFilter && (
        <div className="p-3 px-4 border-b border-surface-alt bg-tertiary">
          <div className="flex flex-wrap gap-2 mb-2">
            {tags.map((tag) => (
              <div
                key={tag.id}
                className="cursor-pointer"
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

      <div className="flex-1 overflow-y-auto">
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
                  className="border-b border-surface-alt"
                  ref={(el) => {
                    if (el) resultRefs.current.set(post.id, el)
                  }}
                >
                  <div 
                    className="flex items-center justify-between p-3 px-4 cursor-pointer hover:bg-surface-alt transition-colors"
                    onClick={() => handleExpand(post)}
                  >
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 text-xs text-muted mb-1">
                        <span className="font-semibold text-muted">#{post.number}</span>
                        <span className={classSet({
                          "px-1.5 py-0.5 rounded text-xs": true,
                          "bg-surface-alt text-muted": post.status === "open",
                          "bg-info-light text-primary": post.status === "planned",
                          "bg-success-light text-success": post.status === "started",
                          "bg-success-medium text-success": post.status === "completed",
                          "bg-danger-light text-danger": post.status === "declined",
                        })}>
                          {PostStatus.Get(post.status).title}
                        </span>
                        <span className="text-muted">
                          {(post.upvotes || 0) - (post.downvotes || 0)} votes
                        </span>
                        <span className="text-muted">
                          {post.commentsCount} comments
                        </span>
                      </div>
                      <div className="font-medium text-foreground whitespace-nowrap overflow-hidden text-ellipsis">{post.title}</div>
                      {postTags.length > 0 && (
                        <HStack spacing={1} className="mt-1">
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
                      <span onClick={(e) => e.stopPropagation()}>
                        <Button
                          variant="primary"
                          size="small"
                          onClick={() => onSelect(post.number)}
                        >
                          Select
                        </Button>
                      </span>
                      <Icon
                        sprite={isExpanded ? IconChevronUp : IconChevronDown}
                        className="h-5 text-muted"
                      />
                    </HStack>
                  </div>

                  {isExpanded && (
                    <div className="p-3 px-4 bg-tertiary border-t border-surface-alt">
                      {isLoadingExpanded ? (
                        <div className="py-4 text-center">
                          <Loader />
                        </div>
                      ) : expandedData ? (
                        <>
                          <div className="bg-elevated border border-surface-alt rounded-card p-3 mb-3">
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
                            <div className="bg-elevated border border-surface-alt rounded-card p-3">
                              <button
                                className="flex items-center justify-between w-full border-none bg-transparent cursor-pointer p-0"
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
                                    <div key={comment.id} className="p-2 bg-tertiary rounded-card [&_.c-avatar]:w-5 [&_.c-avatar]:h-5">
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
