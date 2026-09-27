import React, { useState, useMemo, useEffect, useCallback } from "react"
import { Post, Tag } from "@fider/models"
import { actions } from "@fider/services"
import { ShowTag, Button, Input } from "@fider/components"
import { TagListItem } from "./TagListItem"

import { HStack, VStack } from "@fider/components/layout"
import { Trans } from "@lingui/react/macro"

export interface TagsPanelProps {
  post: Post
  tags: Tag[]
  onTagsChanged?: (postNumber: number) => void
  onNextPost?: () => void
}

export const TagsPanel = (props: TagsPanelProps) => {
  const assignableTags = useMemo(() => props.tags.filter((tag) => tag.permissions.assign), [props.tags])
  const canEdit = props.post.permissions.tag && assignableTags.length > 0

  const [isEditing, setIsEditing] = useState(false)
  const [assignedTags, setAssignedTags] = useState(props.tags.filter((t) => props.post.tags.indexOf(t.slug) >= 0))
  const [searchQuery, setSearchQuery] = useState("")
  const [recentlyUsedTags, setRecentlyUsedTags] = useState<Tag[]>([])

  useEffect(() => {
    setAssignedTags(props.tags.filter((t) => props.post.tags.indexOf(t.slug) >= 0))
  }, [props.post.tags, props.tags])

  const assignedTagSlugs = useMemo(() => new Set(assignedTags.map(t => t.slug)), [assignedTags])
  
  useEffect(() => {
    if (isEditing) {
      try {
        const recentTagSlugs = JSON.parse(localStorage.getItem("fider_recent_tags") || "[]")
        const recentTags = assignableTags.filter(tag => recentTagSlugs.includes(tag.slug))
        setRecentlyUsedTags(recentTags.slice(0, 5))
      } catch (e) {
        console.error("Failed to load recent tags", e)
      }
    }
  }, [isEditing, assignableTags])

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

  const assignOrUnassignTag = useCallback(async (tag: Tag) => {
    if (!canEdit || !tag.permissions.assign) return

    const isAssigned = assignedTagSlugs.has(tag.slug)
    let nextAssignedTags: Tag[] = []

    if (isAssigned) {
      const response = await actions.unassignTag(tag.slug, props.post.number)
      if (response.ok) {
        nextAssignedTags = assignedTags.filter(t => t.slug !== tag.slug)
        props.onTagsChanged?.(props.post.number)
      } else {
        return
      }
    } else {
      const response = await actions.assignTag(tag.slug, props.post.number)
      if (response.ok) {
        nextAssignedTags = [...assignedTags, tag]
        saveRecentTag(tag)
        props.onTagsChanged?.(props.post.number)
      } else {
        return
      }
    }

    setAssignedTags(nextAssignedTags)
  }, [canEdit, assignedTags, assignedTagSlugs, props.post.number, props.onTagsChanged])

  const onSubtitleClick = () => {
    if (canEdit) {
      setIsEditing(!isEditing)
      if (!isEditing) {
        setSearchQuery("")
      }
    }
  }

  const clearAllTags = useCallback(async () => {
    if (assignedTags.length === 0) return
    if (!window.confirm("Are you sure you want to remove all tags from this post?")) return
    
    const removedTags: Tag[] = []
    
    for (const tag of assignedTags.filter((tag) => tag.permissions.assign)) {
      const response = await actions.unassignTag(tag.slug, props.post.number)
      if (!response.ok) {
        if (removedTags.length > 0) {
          setAssignedTags(assignedTags.filter(t => !removedTags.includes(t)))
          props.onTagsChanged?.(props.post.number)
        }
        return
      }
      removedTags.push(tag)
    }
    
    setAssignedTags(assignedTags.filter((tag) => !removedTags.includes(tag)))
    props.onTagsChanged?.(props.post.number)
  }, [assignedTags, props.post.number, props.onTagsChanged])

  const recentTagSlugs = useMemo(() => new Set(recentlyUsedTags.map(t => t.slug)), [recentlyUsedTags])

  const filteredTags = useMemo(() => {
    const query = searchQuery.toLowerCase().trim()
    let tags = assignableTags
    
    if (query) {
      tags = tags.filter(
        tag => tag.name.toLowerCase().includes(query) || tag.slug.toLowerCase().includes(query)
      )
    }
    
    return tags
  }, [assignableTags, searchQuery])

  const nonRecentTags = useMemo(() => {
    if (searchQuery.trim()) return filteredTags
    return filteredTags.filter(tag => !recentTagSlugs.has(tag.slug))
  }, [filteredTags, recentTagSlugs, searchQuery])

  if (!canEdit && assignedTags.length === 0) {
    return null
  }

  const tagsList = (
    <div className="flex flex-wrap gap-2 items-center">
      {assignedTags.length > 0 &&
        assignedTags.map((tag) => <ShowTag key={tag.id} tag={tag} link />)}
      {canEdit ? (
        <span className="text-link cursor-pointer whitespace-nowrap" onClick={onSubtitleClick}>
          <Trans id="label.edittags">Edit tags</Trans>
        </span>
      ) : null}
    </div>
  )

  const renderTagsGrid = () => {
    const tagsToShow = searchQuery.trim() ? filteredTags : nonRecentTags
    const showRecent = !searchQuery.trim() && recentlyUsedTags.length > 0

    return (
      <div>
        {showRecent && (
          <div className="mb-2 last:mb-0">
            <span className="block text-xs font-semibold text-muted uppercase tracking-wide mb-1">Recent</span>
            <div className="flex flex-wrap gap-1.5">
              {recentlyUsedTags.map(tag => (
                <TagListItem 
                  key={`recent-${tag.id}`} 
                  tag={tag} 
                  assigned={assignedTagSlugs.has(tag.slug)} 
                  onClick={assignOrUnassignTag} 
                />
              ))}
            </div>
          </div>
        )}
        
        {tagsToShow.length > 0 && (
          <div className="mb-2 last:mb-0">
            {showRecent && <span className="block text-xs font-semibold text-muted uppercase tracking-wide mb-1">All Tags</span>}
            <div className="flex flex-wrap gap-1.5">
              {tagsToShow.map(tag => (
                <TagListItem 
                  key={tag.id} 
                  tag={tag} 
                  assigned={assignedTagSlugs.has(tag.slug)} 
                  onClick={assignOrUnassignTag} 
                />
              ))}
            </div>
          </div>
        )}
      </div>
    )
  }

  const editTagsList = props.tags.length > 0 && (
    <div className="flex flex-col gap-3 w-full">
      <div className="flex items-center gap-2">
        <Input
          field="search"
          className="flex-1"
          placeholder="Search tags..."
          value={searchQuery}
          onChange={setSearchQuery}
        />
        <span className="text-xs text-muted whitespace-nowrap">
          <span className="font-medium">{assignedTags.length}</span>/{props.tags.length}
        </span>
      </div>
      
      {filteredTags.length > 0 ? (
        <div className="w-full max-h-48 overflow-y-auto">
          {renderTagsGrid()}
        </div>
      ) : (
        <div className="text-center text-muted py-3 text-sm">
          <Trans id="label.notags">No tags found</Trans>
        </div>
      )}
      
      <div className="flex items-center justify-between gap-2 pt-2 border-t border-border">
        {assignedTags.length > 0 ? (
          <Button variant="tertiary" size="small" onClick={clearAllTags}>
            <Trans id="action.cleartags">Clear</Trans>
          </Button>
        ) : <div />}
        <div className="flex gap-2">
          <Button variant="secondary" size="small" onClick={onSubtitleClick}>
            <Trans id="action.close">Done</Trans>
          </Button>
          {props.onNextPost && (
            <Button variant="primary" size="small" onClick={props.onNextPost} className="hidden max-lg:inline-flex">
              <Trans id="action.nextPost">Next</Trans>
            </Button>
          )}
        </div>
      </div>
    </div>
  )

  if (!canEdit) {
    return (
      <VStack>
        <HStack spacing={2} className="text-category">
          <Trans id="label.tags">Tags</Trans>
        </HStack>
        {tagsList}
      </VStack>
    )
  }

  return (
    <VStack>
      {!isEditing ? (
        <HStack spacing={2} align="center" className="text-primary text-xs">
          {tagsList}
        </HStack>
      ) : (
        <div className="p-3 border border-border rounded-card shadow-sm w-full">
          {editTagsList}
        </div>
      )}
    </VStack>
  )
}
