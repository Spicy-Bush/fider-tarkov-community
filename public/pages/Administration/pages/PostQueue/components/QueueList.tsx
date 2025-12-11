import React from "react"
import { Button, Loader, Icon, Dropdown } from "@fider/components"
import { VStack, HStack } from "@fider/components/layout"
import { heroiconsRefresh as IconRefresh, heroiconsArrowUpDown as IconSort } from "@fider/icons.generated"
import { Post, ViewerInfo } from "@fider/models"
import { Trans } from "@lingui/react/macro"
import { QueueListItem } from "./QueueListItem"
import { QueueSortOption } from "../hooks/useQueueState"

const sortOptions: { value: QueueSortOption; label: string }[] = [
  { value: "newest", label: "Newest First" },
  { value: "oldest", label: "Oldest First" },
  { value: "most-wanted", label: "Most Votes" },
  { value: "least-wanted", label: "Least Votes" },
]

interface QueueListProps {
  posts: Post[]
  total: number
  page: number
  perPage: number
  isLoading: boolean
  selectedPost: Post | null
  newPostIds: Set<number>
  taggedByOtherIds: Set<number>
  viewers: Map<number, ViewerInfo[]>
  sortOption: QueueSortOption
  onSelectPost: (post: Post) => void
  onRefresh: () => void
  onPrevPage: () => void
  onNextPage: () => void
  onSortChange: (value: QueueSortOption) => void
}

export const QueueList: React.FC<QueueListProps> = ({
  posts,
  total,
  page,
  perPage,
  isLoading,
  selectedPost,
  newPostIds,
  taggedByOtherIds,
  viewers,
  sortOption,
  onSelectPost,
  onRefresh,
  onPrevPage,
  onNextPage,
  onSortChange,
}) => {
  const getViewersForPost = (postId: number): ViewerInfo[] => {
    return viewers.get(postId) || []
  }

  const selectedSort = sortOptions.find((o) => o.value === sortOption) || sortOptions[0]

  return (
    <div className="c-queue-split-view__list">
      <div className="c-queue-split-view__filters">
        <HStack spacing={2}>
          <span className="text-medium">
            <Trans id="queue.title">Untagged Posts</Trans>
          </span>
          {total > 0 && <span className="text-muted text-sm">({total})</span>}
        </HStack>
        <HStack spacing={2}>
          <Dropdown
            renderHandle={
              <span className="c-queue-sort-btn">
                <Icon sprite={IconSort} className="h-4" />
                <span>{selectedSort.label}</span>
              </span>
            }
          >
            {sortOptions.map((o) => (
              <Dropdown.ListItem key={o.value} onClick={() => onSortChange(o.value)}>
                <span className={sortOption === o.value ? "text-semibold" : ""}>{o.label}</span>
              </Dropdown.ListItem>
            ))}
          </Dropdown>
          <Button
            variant="secondary"
            size="small"
            onClick={onRefresh}
            className="c-queue-split-view__refresh-btn"
          >
            <Icon sprite={IconRefresh} />
          </Button>
        </HStack>
      </div>

      <div className="c-queue-split-view__list-content">
        {newPostIds.size > 0 && (
          <button className="c-queue-new-banner" onClick={onRefresh}>
            <Icon sprite={IconRefresh} className="h-4" />
            <span>
              {newPostIds.size} new post{newPostIds.size > 1 ? "s" : ""} available
            </span>
          </button>
        )}
        {isLoading ? (
          <div className="py-8 text-center">
            <Loader />
          </div>
        ) : posts.length === 0 ? (
          <div className="py-8 text-center text-muted">
            <Trans id="queue.empty">No untagged posts found.</Trans>
          </div>
        ) : (
          <VStack spacing={0} divide>
            {posts.map((post) => (
              <QueueListItem
                key={post.id}
                post={post}
                isSelected={selectedPost?.id === post.id}
                onClick={onSelectPost}
                viewers={getViewersForPost(post.id)}
                isTaggedByOther={taggedByOtherIds.has(post.id)}
              />
            ))}
          </VStack>
        )}
      </div>

      {total > posts.length && (
        <div className="c-queue-split-view__pagination">
          <Button
            size="small"
            variant="tertiary"
            disabled={page === 1}
            onClick={onPrevPage}
          >
            Prev
          </Button>
          <span className="text-muted text-sm">Page {page}</span>
          <Button
            size="small"
            variant="tertiary"
            disabled={posts.length < perPage}
            onClick={onNextPage}
          >
            Next
          </Button>
        </div>
      )}
    </div>
  )
}

