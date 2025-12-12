import React from "react"
import { classSet } from "@fider/services"

interface StackProps {
  className?: string
  children: React.ReactNode
  onClick?: () => void
  divide?: boolean
  justify?: "between" | "evenly" | "full" | "center"
  align?: "start" | "center" | "end"
  spacing?: 0 | 1 | 2 | 4 | 6 | 8
}

const spacingMap = {
  0: "",
  1: "gap-1",
  2: "gap-2",
  4: "gap-4",
  6: "gap-6",
  8: "gap-8",
}

const divideMapX = {
  0: "",
  2: "divide-x divide-border [&>*]:px-2 [&>*:first-child]:pl-0 [&>*:last-child]:pr-0",
  4: "divide-x divide-border [&>*]:px-4 [&>*:first-child]:pl-0 [&>*:last-child]:pr-0",
  6: "divide-x divide-border [&>*]:px-6 [&>*:first-child]:pl-0 [&>*:last-child]:pr-0",
}

const divideMapY = {
  0: "",
  2: "divide-y divide-border [&>*]:py-2 [&>*:first-child]:pt-0 [&>*:last-child]:pb-0",
  4: "divide-y divide-border [&>*]:py-4 [&>*:first-child]:pt-0 [&>*:last-child]:pb-0",
  6: "divide-y divide-border [&>*]:py-6 [&>*:first-child]:pt-0 [&>*:last-child]:pb-0",
}

const Stack = (props: StackProps, dir: "x" | "y") => {
  const spacing = props.spacing === undefined ? 2 : props.spacing
  
  const className = classSet({
    [`${props.className}`]: props.className,
    "flex": true,
    "flex-row": dir === "x",
    "flex-col": dir === "y",
    [spacingMap[spacing] || ""]: spacing > 0 && !props.divide,
    [dir === "x" ? (divideMapX[spacing as keyof typeof divideMapX] || "") : (divideMapY[spacing as keyof typeof divideMapY] || "")]: spacing > 0 && !!props.divide,
    "justify-between": props.justify === "between",
    "justify-evenly": props.justify === "evenly",
    "[&>*]:flex-1": props.justify === "full",
    "justify-center": props.justify === "center",
    "items-start": props.align === "start",
    "items-center": props.align === "center" || (dir === "x" && props.align === undefined),
    "items-end": props.align === "end",
  })

  return (
    <div onClick={props.onClick} className={className}>
      {props.children}
    </div>
  )
}

export const HStack = (props: StackProps) => {
  return Stack(props, "x")
}

export const VStack = (props: StackProps) => {
  return Stack(props, "y")
}
