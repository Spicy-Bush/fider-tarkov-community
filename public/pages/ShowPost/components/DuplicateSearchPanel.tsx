import React from "react"
import { Loader } from "@fider/components"
import { Tag } from "@fider/models"
import { PostQueueDuplicateSearch } from "@fider/pages/Administration/pages/PostQueue/components/PostQueueDuplicateSearch"

interface DuplicateSearchPanelProps {
  excludePostNumber: number
  tags: Tag[]
  isLoading: boolean
  onSelect: (postNumber: number) => void
  onCancel: () => void
}

export const DuplicateSearchPanel: React.FC<DuplicateSearchPanelProps> = ({
  excludePostNumber,
  tags,
  isLoading,
  onSelect,
  onCancel,
}) => {
  return (
    <>
      <div className="c-duplicate-search-backdrop" onClick={onCancel} />
      <div className="c-duplicate-search-overlay">
        {isLoading ? (
          <div className="c-duplicate-search-overlay__loading">
            <Loader />
          </div>
        ) : (
          <PostQueueDuplicateSearch
            excludePostNumber={excludePostNumber}
            tags={tags}
            onSelect={onSelect}
            onCancel={onCancel}
          />
        )}
      </div>
    </>
  )
}

