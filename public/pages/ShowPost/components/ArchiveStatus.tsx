import React from "react"
import { Post, isPostArchived } from "@fider/models"
import { Moment } from "@fider/components"
import { useFider } from "@fider/hooks"
import { Trans } from "@lingui/react/macro"
import { Icon } from "@fider/components"
import { heroiconsArchive as IconArchive } from "@fider/icons.generated"

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
    <div className="mt-4 rounded-card border border-border bg-surface-alt overflow-hidden">
      <div className="flex items-center gap-2 px-4 py-3 bg-tertiary border-b border-border">
        <div className="flex items-center justify-center w-6 h-6 rounded-full bg-muted/20">
          <Icon height="14" width="14" sprite={IconArchive} className="text-muted" />
        </div>
        <span className="font-medium text-foreground">
          <Trans id="showpost.archived.title">This post has been archived</Trans>
        </span>
      </div>
      
      <div className="px-4 py-3 text-sm text-foreground border-b border-border">
        <Trans id="showpost.archived.message">
          This post was archived due to inactivity, or is no longer relevant. Vote or comment to bring it back.
        </Trans>
      </div>
      
      {post.archivedSettings && (
        <div className="px-4 py-2 text-xs text-muted">
          <Trans id="showpost.archived.archivedat">Archived</Trans>{" "}
          <Moment locale={fider.currentLocale} date={post.archivedSettings.archivedAt} />
        </div>
      )}
    </div>
  )
}

