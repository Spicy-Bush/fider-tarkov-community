import "./ShowTag.scss"

import React from "react"
import { Tag } from "@fider/models"
import { classSet } from "@fider/services"
import { heroiconsShieldcheck as ShieldCheck, heroiconsCheck as IconCheck } from "@fider/icons.generated"
import { Icon } from "../common"

interface TagProps {
  tag: Tag
  circular?: boolean
  link?: boolean
  size?: "default" | "mini"
  selectable?: boolean
  selected?: boolean
}

export const ShowTag = (props: TagProps) => {
  const className = classSet({
    "c-tag": true,
    "c-tag--circular": props.circular === true,
    "c-tag--mini": props.size === "mini",
    "c-tag--selectable": props.selectable === true,
    "c-tag--selected": props.selected === true,
  })

  return (
    <a
      href={props.link && props.tag.slug ? `/?tags=${props.tag.slug}` : undefined}
      title={`${props.tag.name}${props.tag.isPublic ? "" : " (Private)"}`}
      className={className}
    >
      <span
        className="c-tag__color"
        style={{
          backgroundColor: `#${props.tag.color}`,
        }}
      ></span>
      {!props.tag.isPublic && !props.circular && <Icon height="14" width="14" sprite={ShieldCheck} className="mr-1" />}
      {props.circular ? "" : props.tag.name || "Tag"}
      {props.selectable && (
        <span className="c-tag__check">
          {props.selected && <Icon sprite={IconCheck} className="c-tag__check-icon" />}
        </span>
      )}
    </a>
  )
}
