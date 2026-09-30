import React, { useMemo } from "react"
import * as markdown from "@fider/services/markdown"
import { truncate } from "@fider/services/utils"

// import "./Markdown.scss"

interface MarkdownProps {
  className?: string
  text?: string
  maxLength?: number
  style: "full" | "plainText"
  embedImages?: boolean
}

export const Markdown = React.memo((props: MarkdownProps) => {
  const content = useMemo(() => {
    if (!props.text) return null

    if (props.style === "full") {
      return markdown.full(props.text, props.embedImages || false)
    }

    const text = markdown.plainText(props.text)
    return props.maxLength ? truncate(text, props.maxLength) : text
  }, [props.text, props.style, props.maxLength, props.embedImages])

  if (!content) return null

  const className = `c-markdown wrap-break-word ${props.className || ""}`
  const tagName = props.style === "plainText" ? "p" : "div"

  return React.createElement(tagName, { className }, content)
})
