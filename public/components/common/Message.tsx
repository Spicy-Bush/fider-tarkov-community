import "./Message.scss"

import React from "react"
import { classSet } from "@fider/services"
import { heroiconsCheckCircle as IconCheckCircle, heroiconsExclamationCircle as IconExclamationCircle, heroiconsExclamation as IconExclamation } from "@fider/icons.generated"
import { HStack } from "@fider/components/layout"
import { Icon } from "./Icon"

interface MessageProps {
  children?: React.ReactNode
  type: "success" | "warning" | "error"
  className?: string
  alignment?: "center"
  showIcon?: boolean
}

export const Message: React.FunctionComponent<MessageProps> = (props) => {
  const className = classSet({
    "c-message": true,
    [`c-message--${props.type}`]: true,
    [`c-message--icon`]: props.showIcon === true,
    [`${props.className}`]: props.className,
  })

  const icon = props.type === "error" ? IconExclamation : props.type === "warning" ? IconExclamationCircle : IconCheckCircle

  return (
    <HStack className={className} spacing={2} justify={props.alignment}>
      {props.showIcon === true && <Icon className="h-5" sprite={icon} />}
      <span>{props.children}</span>
    </HStack>
  )
}
