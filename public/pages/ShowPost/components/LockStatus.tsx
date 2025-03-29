import React from "react"
import { Post } from "@fider/models"
import { UserName, Moment } from "@fider/components"
import { useFider } from "@fider/hooks"
import { Trans } from "@lingui/react/macro"
import { Icon } from "@fider/components"
import HeroIconLock from "@fider/assets/images/heroicons-lock.svg"

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
    <div className="c-locked-post mt-2">
      <div className="c-locked-post__header">
        <div className="c-locked-post__icon">
          <Icon height="14" width="14" sprite={HeroIconLock} />
        </div>
        
        <div className="c-locked-post__title">
          <Trans id="showpost.locked.title">This post has been locked</Trans>
        </div>
      </div>
      
      {post.lockedSettings.lockMessage && (
        <div className="c-locked-post__reason">
          {post.lockedSettings.lockMessage}
        </div>
      )}
      
      <div className="c-locked-post__footer">
        <div className="c-locked-post__by">
          <Trans id="showpost.locked.by">Locked by</Trans>{" "}
          <UserName user={post.lockedSettings.lockedBy} />
        </div>
        <div>
          <Moment locale={fider.currentLocale} date={post.lockedSettings.lockedAt} />
        </div>
      </div>
    </div>
  )
}