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
  const isMini = props.size === "mini"
  const isCircular = props.circular === true
  
  const innerClassName = classSet({
    "inline-flex justify-center items-center text-foreground font-medium": true,
    "text-[13px] px-2.5 py-1.5 tag-clipped-inner": !isMini && !isCircular,
    "text-[11px] px-1.5 py-0.5 tag-clipped-inner": isMini && !isCircular,
    "min-h-0 min-w-0 overflow-hidden rounded-full p-1.5": isCircular,
    "bg-surface-alt": !props.selected,
    "bg-success-medium": props.selected === true,
  })

  const outerClassName = classSet({
    "inline-block": !isCircular,
    "bg-border tag-clipped p-px": !isCircular,
    "rounded-full": isCircular,
    "cursor-pointer transition-all duration-50 hover:bg-border-strong": props.selectable === true && !props.selected,
    "hover:bg-success-dark": props.selected === true,
    "[&[href]]:opacity-90 [&[href]]:transition-opacity [&[href]]:duration-100 [&[href]:hover]:opacity-100": true,
    "[&:focus]:!outline-none [&:focus-visible]:!outline-none [&:focus]:!ring-0 [&:focus-visible]:!ring-0": true,
  })

  const colorClassName = classSet({
    "rounded-full inline-block shrink-0": true,
    "w-3 h-3 mr-1.5": !isMini && !isCircular,
    "w-2 h-2 mr-1": isMini,
    "w-3 h-3 mr-0": isCircular,
  })

  const checkClassName = classSet({
    "inline-flex items-center justify-center shrink-0": true,
    "w-3.5 h-3.5 ml-1.5": !isMini,
    "w-3 h-3 ml-1": isMini,
  })

  return (
    <a
      href={props.link && props.tag.slug ? `/?tags=${props.tag.slug}` : undefined}
      title={`${props.tag.name}${props.tag.isPublic ? "" : " (Private)"}`}
      className={outerClassName}
      style={{ outline: 'none', outlineColor: 'transparent' }}
      onFocus={(e) => e.currentTarget.style.outline = 'none'}
    >
      <span className={innerClassName}>
        <span
          className={colorClassName}
          style={{
            backgroundColor: `#${props.tag.color}`,
          }}
        ></span>
        {!props.tag.isPublic && !isCircular && <Icon height="14" width="14" sprite={ShieldCheck} className="mr-1" />}
        {isCircular ? "" : props.tag.name || "Tag"}
        {props.selectable && (
          <span className={checkClassName}>
            {props.selected && <Icon sprite={IconCheck} className="w-3.5 h-3.5 text-primary" />}
          </span>
        )}
      </span>
    </a>
  )
}
