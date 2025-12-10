import React, { useMemo } from "react"
import { markdown, truncate } from "@fider/services"

import "./Markdown.scss"

interface MarkdownProps {
  className?: string
  text?: string
  maxLength?: number
  style: "full" | "plainText"
}

export const Markdown = React.memo((props: MarkdownProps) => {
  const html = useMemo(() => {
    if (!props.text) return null
    const parsed = markdown[props.style](props.text)
    return props.maxLength ? truncate(parsed, props.maxLength) : parsed
  }, [props.text, props.style, props.maxLength])

  if (!html) return null

  const className = `c-markdown ${props.className || ""}`
  const tagName = props.style === "plainText" ? "p" : "div"

  return React.createElement(tagName, {
    className,
    dangerouslySetInnerHTML: { __html: html },
  })
})
