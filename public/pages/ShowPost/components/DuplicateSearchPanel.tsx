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
      <div 
        className="fixed inset-0 bg-black/50 z-1200 animate-[fadeIn_0.2s_ease]" 
        onClick={onCancel} 
      />
      <div className="fixed top-0 right-0 bottom-0 w-full max-w-[600px] z-1201 bg-elevated shadow-xl overflow-y-auto animate-[slideInFromRight_0.2s_ease] max-sm:max-w-full max-sm:left-0">
        {isLoading ? (
          <div className="flex items-center justify-center h-full min-h-[300px]">
            <Loader />
          </div>
        ) : (
          <div className="h-full">
            <PostQueueDuplicateSearch
              excludePostNumber={excludePostNumber}
              tags={tags}
              onSelect={onSelect}
              onCancel={onCancel}
            />
          </div>
        )}
      </div>
    </>
  )
}

