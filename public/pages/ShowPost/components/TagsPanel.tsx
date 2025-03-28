import React, { useState, useMemo, useEffect } from "react"
import { Post, Tag } from "@fider/models"
import { actions } from "@fider/services"
import { ShowTag, Button, Input, Icon } from "@fider/components"
import { TagListItem } from "./TagListItem"
import { useFider } from "@fider/hooks"

import { HStack, VStack } from "@fider/components/layout"
import { Trans } from "@lingui/react/macro"
import IconSearch from "@fider/assets/images/heroicons-search.svg"
import IconX from "@fider/assets/images/heroicons-x.svg"

export interface TagsPanelProps {
  post: Post
  tags: Tag[]
}

export const TagsPanel = (props: TagsPanelProps) => {
  const fider = useFider()
  const canEdit = fider.session.isAuthenticated && (fider.session.user.isCollaborator || fider.session.user.isModerator || fider.session.user.isAdministrator) && props.tags.length > 0

  const [isEditing, setIsEditing] = useState(false)
  const [assignedTags, setAssignedTags] = useState(props.tags.filter((t) => props.post.tags.indexOf(t.slug) >= 0))
  const [searchQuery, setSearchQuery] = useState("")
  const [recentlyUsedTags, setRecentlyUsedTags] = useState<Tag[]>([])
  
  useEffect(() => {
    if (isEditing) {
      try {
        const recentTagSlugs = JSON.parse(localStorage.getItem("fider_recent_tags") || "[]")
        const recentTags = props.tags.filter(tag => recentTagSlugs.includes(tag.slug))
        setRecentlyUsedTags(recentTags.slice(0, 5))
      } catch (e) {
        console.error("Failed to load recent tags", e)
      }
    }
  }, [isEditing, props.tags])

  const saveRecentTag = (tag: Tag) => {
    try {
      const recentTagSlugs = JSON.parse(localStorage.getItem("fider_recent_tags") || "[]")
      const updatedRecentTags = [
        tag.slug,
        ...recentTagSlugs.filter((slug: string) => slug !== tag.slug)
      ].slice(0, 10)
      
      localStorage.setItem("fider_recent_tags", JSON.stringify(updatedRecentTags))
    } catch (e) {
      console.error("Failed to save recent tags", e)
    }
  }

  const assignOrUnassignTag = async (tag: Tag) => {
    const idx = assignedTags.indexOf(tag)
    let nextAssignedTags: Tag[] = []

    if (idx >= 0) {
      const response = await actions.unassignTag(tag.slug, props.post.number)
      if (response.ok) {
        nextAssignedTags = [...assignedTags]
        nextAssignedTags.splice(idx, 1)
      }
    } else {
      const response = await actions.assignTag(tag.slug, props.post.number)
      if (response.ok) {
        nextAssignedTags = [...assignedTags, tag]
        saveRecentTag(tag)
      }
    }

    setAssignedTags(nextAssignedTags)
  }

  const onSubtitleClick = () => {
    if (canEdit) {
      setIsEditing(!isEditing)
      if (!isEditing) {
        setSearchQuery("")
      }
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      setIsEditing(false)
    }
  }

  const clearAllTags = async () => {
    if (assignedTags.length === 0) return
    if (!window.confirm("Are you sure you want to remove all tags from this post?")) return
    const promises = assignedTags.map(tag => actions.unassignTag(tag.slug, props.post.number))
    await Promise.all(promises)
    
    setAssignedTags([])
  }

  const filteredTags = useMemo(() => {
    if (!searchQuery.trim()) return props.tags
    const query = searchQuery.toLowerCase()
    return props.tags.filter(
      tag => tag.name.toLowerCase().includes(query) || tag.slug.toLowerCase().includes(query)
    )
  }, [props.tags, searchQuery])

  const groupedTags = useMemo(() => {
    if (searchQuery.trim()) return null
    
    const groups: { [key: string]: Tag[] } = {}
    props.tags.forEach(tag => {
      const firstChar = tag.name.charAt(0).toUpperCase()
      if (!groups[firstChar]) groups[firstChar] = []
      groups[firstChar].push(tag)
    })
    return groups
  }, [props.tags, searchQuery])

  if (!canEdit && assignedTags.length === 0) {
    return null
  }

  const tagsList = (
    <HStack spacing={2} align="center" className="c-tags__list">
      {assignedTags.length > 0 &&
        assignedTags.map((tag) => <ShowTag key={tag.id} tag={tag} link />)}
      {canEdit && (
        <HStack spacing={1} align="center" className="clickable" onClick={onSubtitleClick}>
          <span>
            <Trans id="label.edittags">Edit tags</Trans>
          </span>
        </HStack>
      )}
    </HStack>
  )

  const renderTagsGrid = () => {
    if (groupedTags && !searchQuery.trim()) {
      return (
        <>
          {recentlyUsedTags.length > 0 && (
            <div className="mb-4">
              <div className="font-bold text-sm text-gray-600 mb-1">
                <Trans id="label.recent">Recently Used</Trans>
              </div>
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                {recentlyUsedTags.map(tag => (
                  <TagListItem 
                    key={`recent-${tag.id}`} 
                    tag={tag} 
                    assigned={assignedTags.indexOf(tag) >= 0} 
                    onClick={assignOrUnassignTag} 
                  />
                ))}
              </div>
            </div>
          )}
          
          {Object.entries(groupedTags).sort().map(([letter, tags]) => (
            <div key={letter} className="mb-4">
              <div className="font-bold text-sm text-gray-600 mb-1">{letter}</div>
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                {tags.map(tag => (
                  <TagListItem 
                    key={tag.id} 
                    tag={tag} 
                    assigned={assignedTags.indexOf(tag) >= 0} 
                    onClick={assignOrUnassignTag} 
                  />
                ))}
              </div>
            </div>
          ))}
        </>
      )
    }
    
    return (
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 mt-2">
        {filteredTags.map(tag => (
          <TagListItem 
            key={tag.id} 
            tag={tag} 
            assigned={assignedTags.indexOf(tag) >= 0} 
            onClick={assignOrUnassignTag} 
          />
        ))}
      </div>
    )
  }

  const editTagsList = props.tags.length > 0 && (
    <VStack spacing={2} className="w-full">
      <div onKeyDown={handleKeyDown} className="w-full">
        <HStack spacing={2} justify="between" className="w-full">
          <div className="relative flex-grow">
            <Input
              field="search"
              className="w-full pr-9 pl-3"
              placeholder="Search tags..."
              value={searchQuery}
              onChange={setSearchQuery}
            />
            {searchQuery && (
              <button 
                className="absolute right-3 top-1/2 transform -translate-y-1/2 text-gray-500 hover:text-gray-700"
                onClick={() => setSearchQuery("")}
              >
                <Icon sprite={IconX} className="h-4 w-4" />
              </button>
            )}
            {!searchQuery && (
              <Icon
                sprite={IconSearch}
                className="absolute right-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-500"
              />
            )}
          </div>
          
          <div className="text-sm text-gray-500">
            <span className="font-medium">{assignedTags.length}</span> / {props.tags.length}
          </div>
        </HStack>
      </div>
      
      {filteredTags.length > 0 ? (
        <div className="w-full max-h-64 overflow-y-auto pr-1">
          {renderTagsGrid()}
        </div>
      ) : (
        <div className="text-center text-gray-500 py-4">
          <Trans id="label.notags">No tags found</Trans>
        </div>
      )}
      
      <HStack justify="between" className="w-full">
        <div>
          {assignedTags.length > 0 && (
            <Button variant="danger" size="small" onClick={clearAllTags}>
              <Trans id="action.cleartags">Clear All Tags</Trans>
            </Button>
          )}
        </div>
        <Button variant="secondary" size="small" onClick={onSubtitleClick}>
          <Trans id="action.close">Close</Trans>
        </Button>
      </HStack>
    </VStack>
  )

  if (fider.isReadOnly) {
    return (
      <VStack className="c-tags__container">
        <HStack spacing={2} className="text-category">
          <Trans id="label.tags">Tags</Trans>
        </HStack>
        {tagsList}
      </VStack>
    )
  }

  return (
    <VStack className="c-tags__container">
      {!isEditing ? (
        <HStack spacing={2} align="center" className="text-primary-base text-xs">
          {tagsList}
        </HStack>
      ) : (
        <div className="p-3 border rounded-md shadow-sm w-full">
          <HStack justify="between" className="mb-3">
            <h3 className="text-sm font-medium">
              <Trans id="label.managetags">Manage Tags</Trans>
            </h3>
            {assignedTags.length > 0 && (
              <div className="text-xs px-2 py-1 bg-blue-100 text-blue-800 rounded-full">
                {assignedTags.length} <Trans id="label.tagsselected">selected</Trans>
              </div>
            )}
          </HStack>
          {editTagsList}
        </div>
      )}
    </VStack>
  )
}