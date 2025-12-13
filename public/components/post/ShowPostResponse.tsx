import React from "react"
import { PostResponse, PostStatus } from "@fider/models"
import { Icon, Markdown } from "@fider/components"
import { heroiconsDuplicate as HeroIconDuplicate, heroiconsCheckCircle as HeroIconCheck, heroiconsSparklesOutline as HeroIconSparkles, heroiconsThumbsup as HeroIconThumbsUp, heroiconsThumbsdown as HeroIconThumbsDown } from "@fider/icons.generated"
import { HStack, VStack } from "../layout"
import { timeSince } from "@fider/services"

interface PostResponseProps {
  status: string
  response: PostResponse | null
  small?: boolean
}

const getResponseContainerStyles = (status: PostStatus): { bg: string; border: string; divider: string } => {
  switch (status) {
    case PostStatus.Declined:
      return { bg: "bg-danger-light", border: "border-danger-light", divider: "border-danger-medium/50" }
    case PostStatus.Duplicate:
      return { bg: "bg-warning-light", border: "border-warning-light", divider: "border-warning-medium/50" }
    case PostStatus.Completed:
      return { bg: "bg-success-light", border: "border-success-light", divider: "border-success-medium/50" }
    case PostStatus.Planned:
      return { bg: "bg-info-light", border: "border-primary", divider: "border-primary/20" }
    default:
      return { bg: "bg-success-light", border: "border-success-light", divider: "border-success-medium/50" }
  }
}

export const ResponseDetails = (props: PostResponseProps): JSX.Element | null => {
  const status = PostStatus.Get(props.status)

  if (!props.response || status === PostStatus.Open || status === PostStatus.Archived) {
    return null
  }

  const { bg, border, divider } = getResponseContainerStyles(status)

  return (
    <div className={`${bg} p-4 border ${border} rounded`}>
      <div className="flex items-start justify-between gap-4 mb-3">
        <ResponseLozenge response={props.response} status={props.status} />
        <span className="text-sm text-muted shrink-0">{timeSince("en", new Date(), props.response.respondedAt, "date")}</span>
      </div>
      {props.response?.text && (
        <div className="text-foreground">
          <Markdown text={props.response.text} style="full" />
        </div>
      )}

      {status === PostStatus.Duplicate && props.response.original && (
        <div className={`mt-3 pt-3 border-t ${divider}`}>
          <span className="text-muted text-sm">Original post: </span>
          <a className="text-link" href={`/posts/${props.response.original.number}/${props.response.original.slug}`}>
            {props.response.original.title}
          </a>
        </div>
      )}
    </div>
  )
}

const getLozengeProps = (status: PostStatus): { icon: SpriteSymbol; bg: string; color: string; border: string } => {
  switch (status) {
    case PostStatus.Declined:
      return { icon: HeroIconThumbsDown, bg: "bg-danger-light", color: "text-danger", border: "border-danger" }
    case PostStatus.Duplicate:
      return { icon: HeroIconDuplicate, bg: "bg-warning-light", color: "text-warning", border: "border-warning" }
    case PostStatus.Completed:
      return { icon: HeroIconCheck, bg: "bg-success-medium", color: "text-success", border: "border-success" }
    case PostStatus.Planned:
      return { icon: HeroIconThumbsUp, bg: "bg-info-light", color: "text-primary", border: "border-primary" }
    default:
      return { icon: HeroIconSparkles, bg: "bg-success-light", color: "text-success", border: "border-success" }
  }
}

export const ResponseLozenge = (props: PostResponseProps): JSX.Element | null => {
  const status = PostStatus.Get(props.status)
  const { icon, bg, color, border } = getLozengeProps(status)

  if (status === PostStatus.Open || status === PostStatus.Archived) {
    return <div />
  }

  return (
    <div className={`inline-block ${border.replace('border-', 'bg-')} tag-clipped p-px`}>
      <div className={`${bg} tag-clipped-inner`}>
        <HStack align="start" className={`${color} p-1 px-3`}>
          {!props.small && <Icon sprite={icon} className={`h-5 c-status-col--${status.value}`} />}
          <span className={`c-status-col--${status.value} ${props.small ? "text-sm" : "text-semibold"}`}>{status.title}</span>
        </HStack>
      </div>
    </div>
  )
}
