import React from "react"
import { Tag } from "@fider/models"
import { ShowTag } from "@fider/components"
import { HStack } from "@fider/components/layout"

interface TagListItemProps {
  tag: Tag
  assigned: boolean
  onClick: (tag: Tag) => void
}

export const TagListItem = (props: TagListItemProps) => {
  const onClick = () => {
    props.onClick(props.tag)
  }

  const getTagTooltip = () => {
    return props.assigned ? `${props.tag.name} (currently assigned)` : props.tag.name
  }

  return (
    <div title={getTagTooltip()} className="w-full">
      <HStack 
        className={`
          clickable rounded py-1 px-2 transition-all duration-150
          ${props.assigned 
            ? 'bg-blue-50 border border-blue-200' 
            : 'hover:border border-transparent'
          }
        `} 
        onClick={onClick}
      >
        <ShowTag tag={props.tag} />
      </HStack>
    </div>
  )
}