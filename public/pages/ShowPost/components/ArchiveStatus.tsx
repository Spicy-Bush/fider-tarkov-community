import React from "react"
import { Post, isPostArchived } from "@fider/models"
import { Moment } from "@fider/components"
import { useFider } from "@fider/hooks"
import { Trans } from "@lingui/react/macro"
import { Icon } from "@fider/components"
import { heroiconsInbox as IconArchive } from "@fider/icons.generated"

interface ArchiveStatusProps {
  post: Post
}

export const ArchiveStatus = (props: ArchiveStatusProps) => {
  const { post } = props
  const fider = useFider()

  if (!isPostArchived(post)) {
    return null
  }

  return (
    <div className="c-archived-post mt-2">
      <div className="c-archived-post__header">
        <div className="c-archived-post__icon">
          <Icon height="14" width="14" sprite={IconArchive} />
        </div>
        
        <div className="c-archived-post__title">
          <Trans id="showpost.archived.title">This post has been archived</Trans>
        </div>
      </div>
      
      <div className="c-archived-post__message">
        <Trans id="showpost.archived.message">
          This post was archived due to inactivity, or is no longer relevant. Vote or comment to bring it back.
        </Trans>
      </div>
      
      {post.archivedSettings && (
        <div className="c-archived-post__footer">
          <Moment locale={fider.currentLocale} date={post.archivedSettings.archivedAt} />
        </div>
      )}
    </div>
  )
}

