import React, { useState, useMemo, useEffect, useCallback } from "react"
import { Post, Tag, UserRole } from "@fider/models"
import { actions, postPermissions, permissions } from "@fider/services"
import { ShowTag, Button, Input } from "@fider/components"
import { TagListItem } from "./TagListItem"
import { useFider } from "@fider/hooks"

import { HStack, VStack } from "@fider/components/layout"
import { Trans } from "@lingui/react/macro"

import "./TagsPanel.scss"

export interface TagsPanelProps {
  post: Post
  tags: Tag[]
  onTagsChanged?: (postNumber: number) => void
  onNextPost?: () => void
}

export const TagsPanel = (props: TagsPanelProps) => {
  const fider = useFider()
  const isHelper = fider.session.isAuthenticated && permissions.hasRole(fider.session.user.role, UserRole.Helper) && !fider.session.user.isCollaborator && !fider.session.user.isModerator && !fider.session.user.isAdministrator
  const canEdit = postPermissions.canTag() && props.tags.length > 0

  const helperCanEditTags = useMemo(() => {
    if (!isHelper) return true

    const now = new Date()
    const postCreatedAt = new Date(props.post.createdAt)
    const postAgeDays = (now.getTime() - postCreatedAt.getTime()) / (1000 * 60 * 60 * 24)
    if (postAgeDays > 7) return false

    if (!props.post.tagDates) return true
    
    try {
      const tagDatesArray = JSON.parse(props.post.tagDates);
      if (!tagDatesArray || tagDatesArray.length === 0) return true;
      
      const oldestDate = tagDatesArray.reduce((oldest: Date | null, current: { slug: string; created_at: string }) => {
        const currentDate = new Date(current.created_at);
        return oldest && oldest < currentDate ? oldest : currentDate;
      }, null);
      
      if (!oldestDate) return true;
      
      const hoursDiff = (now.getTime() - oldestDate.getTime()) / (1000 * 60 * 60);
      return hoursDiff <= 24;
    } catch (e) {
      return false;
    }
  }, [isHelper, props.post.createdAt, props.post.tagDates]);

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

  const assignOrUnassignTag = useCallback(async (tag: Tag) => {
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
  }, [assignedTags, assignedTagSlugs, props.post.number, props.onTagsChanged])

  const onSubtitleClick = () => {
    if ((canEdit && !isHelper) || (isHelper && helperCanEditTags)) {
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
    
    for (const tag of assignedTags) {
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
    
    setAssignedTags([])
    props.onTagsChanged?.(props.post.number)
  }, [assignedTags, props.post.number, props.onTagsChanged])

  const recentTagSlugs = useMemo(() => new Set(recentlyUsedTags.map(t => t.slug)), [recentlyUsedTags])

  const filteredTags = useMemo(() => {
    const query = searchQuery.toLowerCase().trim()
    let tags = props.tags
    
    if (query) {
      tags = tags.filter(
        tag => tag.name.toLowerCase().includes(query) || tag.slug.toLowerCase().includes(query)
      )
    }
    
    return tags
  }, [props.tags, searchQuery])

  const nonRecentTags = useMemo(() => {
    if (searchQuery.trim()) return filteredTags
    return filteredTags.filter(tag => !recentTagSlugs.has(tag.slug))
  }, [filteredTags, recentTagSlugs, searchQuery])

  if (!canEdit && assignedTags.length === 0) {
    return null
  }

  const tagsList = (
    <HStack spacing={2} align="center" className="c-tags__list">
      {assignedTags.length > 0 &&
        assignedTags.map((tag) => <ShowTag key={tag.id} tag={tag} link />)}
      {(canEdit && !isHelper) || (isHelper && helperCanEditTags) ? (
        <HStack spacing={1} align="center" className="clickable" onClick={onSubtitleClick}>
          <span>
            <Trans id="label.edittags">Edit tags</Trans>
          </span>
        </HStack>
      ) : null}
    </HStack>
  )

  const renderTagsGrid = () => {
    const tagsToShow = searchQuery.trim() ? filteredTags : nonRecentTags
    const showRecent = !searchQuery.trim() && recentlyUsedTags.length > 0

    return (
      <div className="c-tags-selector">
        {showRecent && (
          <div className="c-tags-selector__section">
            <span className="c-tags-selector__label">Recent</span>
            <div className="c-tags-selector__tags">
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
          <div className="c-tags-selector__section">
            {showRecent && <span className="c-tags-selector__label">All Tags</span>}
            <div className="c-tags-selector__tags">
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
    <VStack spacing={2} className="w-full">
      <div className="w-full">
        <HStack spacing={2} justify="between" className="w-full">
          <div className="relative flex-grow">
            <Input
              field="search"
              className="w-full pr-9 pl-3"
              placeholder="Search tags..."
              value={searchQuery}
              onChange={setSearchQuery}
            />
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
        <HStack spacing={2}>
          <Button variant="secondary" size="small" onClick={onSubtitleClick}>
            <Trans id="action.close">Close</Trans>
          </Button>
          {props.onNextPost && (
            <Button variant="primary" size="small" onClick={props.onNextPost} className="c-tags__next-post-btn">
              <Trans id="action.nextPost">Next Post</Trans>
            </Button>
          )}
        </HStack>
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
          {editTagsList}
        </div>
      )}
    </VStack>
  )
}