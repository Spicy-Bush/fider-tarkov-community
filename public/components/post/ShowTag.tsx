import "./ShowTag.scss"

import React from "react"
import { Tag } from "@fider/models"
import { classSet } from "@fider/services"
import { heroiconsShieldcheck as ShieldCheck } from "@fider/icons.generated"
import { Icon } from "../common"

interface TagProps {
  tag: Tag
  circular?: boolean
  link?: boolean
}

export const ShowTag = (props: TagProps) => {
  const className = classSet({
    "c-tag": true,
    "c-tag--circular": props.circular === true,
  })

  return (
    <a
      href={props.link && props.tag.slug ? `/?tags=${props.tag.slug}` : undefined}
      title={`${props.tag.name}${props.tag.isPublic ? "" : " (Private)"}`}
      className={className}
    >
      <span
        style={{
          backgroundColor: `#${props.tag.color}`,
        }}
      ></span>
      {!props.tag.isPublic && !props.circular && <Icon height="14" width="14" sprite={ShieldCheck} className="mr-1" />}
      {props.circular ? "" : props.tag.name || "Tag"}
    </a>
  )
}

