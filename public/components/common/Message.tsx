// import "./Message.scss"

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

const typeClasses = {
  success: "text-success bg-success/10 border-success",
  warning: "text-warning bg-warning/10 border-warning",
  error: "text-danger bg-danger/10 border-danger",
}

export const Message: React.FunctionComponent<MessageProps> = (props) => {
  const className = classSet({
    "p-4 mb-2 rounded": true,
    "border-l-2": props.showIcon === true,
    [typeClasses[props.type]]: true,
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
