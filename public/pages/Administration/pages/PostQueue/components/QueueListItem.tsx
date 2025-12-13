import React, { useCallback, useMemo, memo } from "react"
import { Icon, Avatar, Moment } from "@fider/components"
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
    "p-3 cursor-pointer transition-colors border-l-[3px] border-l-transparent hover:bg-surface-alt": true,
    "bg-info-light border-l-primary hover:bg-info-light": isSelected && !isTaggedByOther,
    "bg-danger-light border-l-danger hover:bg-danger-medium": isTaggedByOther && !isSelected,
    "bg-danger-medium border-l-danger": isTaggedByOther && isSelected,
  })

  const otherViewers = useMemo(() => 
    viewers.filter((v) => v.userId !== Fider.session.user.id),
    [viewers]
  )

  return (
    <div className={className} onClick={handleClick}>
      <div className="flex items-center justify-between mb-1">
        <span className="font-semibold text-sm text-muted">#{post.number}</span>
        {otherViewers.length > 0 && (
          <span
            className="inline-flex items-center gap-0.5 text-xs text-primary bg-info-medium px-1.5 py-0.5 rounded cursor-help tooltip-left"
            data-tooltip={otherViewers.map((v) => v.userName).join(", ")}
          >
            <Icon sprite={IconEye} className="h-3" />
            <span>{otherViewers.length}</span>
          </span>
        )}
      </div>
      <div className="text-sm text-foreground font-medium mb-1 whitespace-nowrap overflow-hidden text-ellipsis">{post.title}</div>
      <div className="flex items-center gap-1 text-xs text-muted [&_.c-avatar]:w-5 [&_.c-avatar]:h-5">
        <Avatar user={post.user} clickable={false} />
        <span>{post.user.name}</span>
        <Moment locale={Fider.currentLocale} date={post.createdAt} />
      </div>
    </div>
  )
})

QueueListItem.displayName = "QueueListItem"
