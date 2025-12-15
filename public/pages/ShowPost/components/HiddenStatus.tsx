import React from "react"
import { Post } from "@fider/models"
import { Trans } from "@lingui/react/macro"
import { Icon } from "@fider/components"
import { heroiconsExclamationCircle as IconHidden } from "@fider/icons.generated"

interface HiddenStatusProps {
  post: Post
}

export const HiddenStatus = (props: HiddenStatusProps) => {
  const { post } = props

  if (!post.moderationPending) {
    return null
  }

  return (
    <div className="mt-4 rounded-card border border-danger-medium bg-danger-light overflow-hidden">
      <div className="flex items-center gap-2 px-4 py-3 bg-danger-light border-b border-danger-medium">
        <div className="flex items-center justify-center w-6 h-6 rounded-full bg-danger-medium/20">
          <Icon height="14" width="14" sprite={IconHidden} className="text-danger" />
        </div>
        <span className="font-medium text-danger">
          <Trans id="showpost.hidden.title">This post is hidden from public view</Trans>
        </span>
      </div>
      
      <div className="px-4 py-3 text-sm text-foreground">
        <Trans id="showpost.hidden.description">
          This content has been flagged and is only visible to moderators.
        </Trans>
      </div>
    </div>
  )
}

