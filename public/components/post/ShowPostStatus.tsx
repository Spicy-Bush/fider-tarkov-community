import React from "react"
import { PostStatus } from "@fider/models"
import { i18n } from "@lingui/core"
import { classSet } from "@fider/services"

interface ShowPostStatusProps {
  status: PostStatus
}

export const ShowPostStatus = (props: ShowPostStatusProps) => {
  const id = `enum.poststatus.${props.status.value}`
  const title = i18n._(id, { message: props.status.title })
  const statusValue = props.status.value

  const className = classSet({
    "inline-block px-2.5 py-1 text-xs font-bold uppercase tracking-wide tag-clipped": true,
    "bg-surface-alt text-muted": statusValue === "open" || statusValue === "archived",
    "bg-warning-light text-warning": statusValue === "duplicate",
    "bg-info-light text-primary": statusValue === "planned",
    "bg-success-light text-success": statusValue === "started",
    "bg-success-medium text-success-dark": statusValue === "completed",
    "bg-danger-light text-danger": statusValue === "declined",
  })

  return <span className={className}>{title}</span>
}

