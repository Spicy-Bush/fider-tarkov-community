import React, { useCallback, useMemo, memo } from "react"
import { Icon, Avatar, Moment } from "@fider/components"
import { HStack } from "@fider/components/layout"
import { Fider, classSet } from "@fider/services"
import { heroiconsEye as IconEye } from "@fider/icons.generated"
import { Post, ViewerInfo } from "@fider/models"

export interface QueueListItemProps {
  post: Post
  isSelected: boolean
  onClick: (post: Post) => void
  viewers: ViewerInfo[]
  isTaggedByOther?: boolean
}

export const QueueListItem: React.FC<QueueListItemProps> = memo(({
  post,
  isSelected,
  onClick,
  viewers,
  isTaggedByOther = false,
}) => {
  const handleClick = useCallback(() => {
    onClick(post)
  }, [onClick, post])

  const className = classSet({
    "c-queue-list-item": true,
    "c-queue-list-item--selected": isSelected,
    "c-queue-list-item--tagged-by-other": isTaggedByOther,
  })

  const otherViewers = useMemo(() => 
    viewers.filter((v) => v.userId !== Fider.session.user.id),
    [viewers]
  )

  return (
    <div className={className} onClick={handleClick}>
      <div className="c-queue-list-item__header">
        <span className="c-queue-list-item__number">#{post.number}</span>
        <span
          className="c-queue-list-item__viewers"
          data-tooltip={otherViewers.length > 0 ? otherViewers.map((v) => v.userName).join(", ") : undefined}
          style={{ visibility: otherViewers.length > 0 ? "visible" : "hidden" }}
        >
          <Icon sprite={IconEye} className="h-3" />
          <span>{otherViewers.length || 1}</span>
        </span>
      </div>
      <div className="c-queue-list-item__title">{post.title}</div>
      <div className="c-queue-list-item__meta">
        <Avatar user={post.user} clickable={false} />
        <span>{post.user.name}</span>
        <Moment locale={Fider.currentLocale} date={post.createdAt} />
      </div>
    </div>
  )
})

QueueListItem.displayName = "QueueListItem"

