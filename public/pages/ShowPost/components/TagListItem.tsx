import React from "react"
import { Tag } from "@fider/models"
import { ShowTag } from "@fider/components"

interface TagListItemProps {
  tag: Tag
  assigned: boolean
  onClick: (tag: Tag) => void
}

export const TagListItem = (props: TagListItemProps) => {
  const onClick = () => {
    props.onClick(props.tag)
  }

  return (
    <button 
      type="button"
      title={props.assigned ? `${props.tag.name} (assigned)` : props.tag.name}
      className="p-0 border-0 bg-transparent cursor-pointer"
      onClick={onClick}
    >
      <ShowTag tag={props.tag} selectable selected={props.assigned} />
    </button>
  )
}
