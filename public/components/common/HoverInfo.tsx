// HoverInfo converted to Tailwind

import React from "react"
import { Icon } from "./Icon"

import { heroiconsInformationCircle as IconInformationCircle } from "@fider/icons.generated"
import { classSet } from "@fider/services"

interface InfoProps {
  text: string
  onClick?: () => void
  href?: string
  target?: "_self" | "_blank" | "_parent" | "_top"
}

export const HoverInfo = (props: InfoProps) => {
  const Elem = props.href ? "a" : "span"
  const classList = classSet({
    "ml-1": true,
    "cursor-pointer": props.onClick !== undefined,
  })
  return (
    <Elem className={classList} data-tooltip={props.text} onClick={props.onClick} href={props.href} target={props.target}>
      <Icon width="15" height="15" className="align-middle" sprite={IconInformationCircle} />
    </Elem>
  )
}
