import React from "react"
import { Tag } from "@fider/models"
import { Icon, ShowTag } from "@fider/components"
import IconCheck from "@fider/assets/images/heroicons-check.svg"
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
        justify="between"
      >
        <ShowTag tag={props.tag} />
        <Icon 
          sprite={IconCheck} 
          className={`h-4 w-4 text-green-600 ml-1 ${!props.assigned && "opacity-0"}`} 
        />
      </HStack>
    </div>
  )
}