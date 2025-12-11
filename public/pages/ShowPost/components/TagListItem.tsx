import React from "react"
import { Tag } from "@fider/models"
import { ShowTag } from "@fider/components"
import { classSet } from "@fider/services"

interface TagListItemProps {
  tag: Tag
  assigned: boolean
  onClick: (tag: Tag) => void
}

export const TagListItem = (props: TagListItemProps) => {
  const onClick = () => {
    props.onClick(props.tag)
  }

  const className = classSet({
    "c-tag-item": true,
    "c-tag-item--assigned": props.assigned,
  })

  return (
    <button 
      type="button"
      title={props.assigned ? `${props.tag.name} (assigned)` : props.tag.name}
      className={className}
      onClick={onClick}
    >
      <ShowTag tag={props.tag} />
      {props.assigned && <span className="c-tag-item__check">✓</span>}
    </button>
  )
}