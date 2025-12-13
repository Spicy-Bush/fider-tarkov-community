import React from "react"
import { Post } from "@fider/models"
import { UserName, Moment } from "@fider/components"
import { useFider } from "@fider/hooks"
import { Trans } from "@lingui/react/macro"
import { Icon } from "@fider/components"
import { heroiconsLock as IconLock } from "@fider/icons.generated"

interface LockStatusProps {
  post: Post
}

export const LockStatus = (props: LockStatusProps) => {
  const { post } = props
  const fider = useFider()

  if (!post.lockedSettings || !post.lockedSettings.locked) {
    return null
  }

  return (
    <div className="mt-4 rounded-card border border-border bg-surface-alt overflow-hidden">
      <div className="flex items-center gap-2 px-4 py-3 bg-tertiary border-b border-border">
        <div className="flex items-center justify-center w-6 h-6 rounded-full bg-muted/20">
          <Icon height="14" width="14" sprite={IconLock} className="text-muted" />
        </div>
        <span className="font-medium text-foreground">
          <Trans id="showpost.locked.title">This post has been locked</Trans>
        </span>
      </div>
      
      {post.lockedSettings.lockMessage && (
        <div className="px-4 py-3 text-sm text-foreground border-b border-border">
          {post.lockedSettings.lockMessage}
        </div>
      )}
      
      <div className="flex items-center justify-between gap-4 px-4 py-2 text-xs text-muted flex-wrap">
        <div>
          <Trans id="showpost.locked.by">Locked by</Trans>{" "}
          <UserName user={post.lockedSettings.lockedBy} />
        </div>
        <Moment locale={fider.currentLocale} date={post.lockedSettings.lockedAt} />
      </div>
    </div>
  )
}