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
    <div className="flex-[0_0_100%] lg:flex-[0_0_400px] lg:min-w-[400px] lg:h-full flex flex-col bg-elevated rounded-panel border border-surface-alt overflow-hidden">
      <div className="flex items-center justify-between p-3 border-b border-surface-alt bg-tertiary shrink-0">
        <HStack spacing={2}>
          <span className="text-medium">
            <Trans id="queue.title">Untagged Posts</Trans>
          </span>
          {total > 0 && <span className="text-muted text-sm">({total})</span>}
        </HStack>
        <HStack spacing={2}>
          <Dropdown
            renderHandle={
              <span className="flex items-center gap-1 px-2 py-1 border border-border rounded-badge bg-elevated text-xs text-foreground cursor-pointer hover:border-border-strong hover:bg-surface-alt transition-colors">
                <Icon sprite={IconSort} className="h-4 text-muted" />
                <span>{selectedSort.label}</span>
              </span>
            }
          >
            {sortOptions.map((o) => (
              <Dropdown.ListItem key={o.value} onClick={() => onSortChange(o.value)}>
                <span className={sortOption === o.value ? "font-semibold" : ""}>{o.label}</span>
              </Dropdown.ListItem>
            ))}
          </Dropdown>
          <Button
            variant="secondary"
            size="small"
            onClick={onRefresh}
            className="w-9 h-9 p-0 shrink-0 flex items-center justify-center [&_svg]:w-4 [&_svg]:h-4"
          >
            <Icon sprite={IconRefresh} />
          </Button>
        </HStack>
      </div>

      <div className="flex-1 overflow-y-auto min-h-0">
        {newPostIds.size > 0 && (
          <button 
            className="flex items-center justify-center gap-2 w-full py-2 px-3 bg-success-light border-none border-b border-success-light text-success font-medium text-sm cursor-pointer hover:bg-success-medium transition-colors [&_svg]:text-success"
            onClick={onRefresh}
          >
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
        <div className="flex items-center justify-between py-2 px-3 border-t border-surface-alt bg-tertiary shrink-0">
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
            disabled={page * perPage >= total}
            onClick={onNextPage}
          >
            Next
          </Button>
        </div>
      )}
    </div>
  )
}
