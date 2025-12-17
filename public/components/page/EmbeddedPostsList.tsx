import React, { useMemo, useState, useEffect } from "react"
import { Post } from "@fider/models"
import { VoteCounter, Icon, Markdown } from "@fider/components"
import { ResponseLozenge } from "@fider/components/post/ShowPostResponse"
import { heroiconsChatAlt2 as IconChatAlt2 } from "@fider/icons.generated"
import { HStack, VStack } from "@fider/components/layout"
import { getVotePosition } from "@fider/components/UserProfile/UserProfileSettings"

interface EmbeddedPostsListProps {
  posts: Post[]
}

const EmbeddedPostItem = (props: { post: Post; votePosition: "left" | "right" }) => {
  const voteSection = (
    <div className="shrink-0 w-16">
      <VStack spacing={2} align="center">
        <VoteCounter post={props.post} />
        {props.post.commentsCount > 0 && (
          <VStack spacing={0} align="center" className="text-muted text-xs mt-1">
            <span>{props.post.commentsCount}</span>
            <Icon sprite={IconChatAlt2} className="h-3 w-3" />
          </VStack>
        )}
      </VStack>
    </div>
  )

  const contentSection = (
    <VStack className="flex-1 min-w-0 max-w-full" spacing={2}>
      <HStack spacing={2} className="flex-wrap min-w-0 max-w-full">
        <div className="shrink-0">
          <ResponseLozenge status={props.post.status} response={props.post.response} small={true} />
        </div>
        {props.post.tags && props.post.tags.length > 0 && (
          <>
            {props.post.tags.map((tagSlug) => (
              <a
                key={tagSlug}
                href={`/?tags=${tagSlug}`}
                className="text-xs px-2 py-1 rounded-badge bg-surface-alt hover-bg-surface-alt-hover text-foreground shrink-0"
              >
                {tagSlug}
              </a>
            ))}
          </>
        )}
      </HStack>
      <a className="text-lg font-medium text-primary hover:text-primary-hover block overflow-wrap-anywhere hyphens-auto" href={`/posts/${props.post.number}/${props.post.slug}`} style={{ wordBreak: 'break-word', overflowWrap: 'anywhere' }}>
        {props.post.title}
      </a>
      {props.post.description && (
        <div className="w-full" style={{ wordBreak: 'break-word', overflowWrap: 'anywhere' }}>
          <Markdown className="text-muted" maxLength={150} text={props.post.description} style="plainText" />
        </div>
      )}
    </VStack>
  )

  const isRight = props.votePosition === "right"

  return (
    <div className={`flex gap-4 items-start min-w-0 max-w-full ${isRight ? "flex-row-reverse" : ""}`}>
      {voteSection}
      {contentSection}
    </div>
  )
}

export const EmbeddedPostsList = (props: EmbeddedPostsListProps) => {
  const [votePosition, setVotePositionState] = useState<"left" | "right">("left")

  useEffect(() => {
    setVotePositionState(getVotePosition())

    const handleStorageChange = () => {
      setVotePositionState(getVotePosition())
    }
    window.addEventListener("storage", handleStorageChange)
    return () => window.removeEventListener("storage", handleStorageChange)
  }, [])

  if (!props.posts || props.posts.length === 0) {
    return <p className="text-muted text-sm">No posts found</p>
  }

  return (
    <VStack spacing={4} divide className="w-full">
      {props.posts.map((post) => (
        <EmbeddedPostItem key={post.id} post={post} votePosition={votePosition} />
      ))}
    </VStack>
  )
}

