import React, { useMemo, useState, useEffect } from "react"
import { Post, Tag, CurrentUser } from "@fider/models"
import { ShowTag, VoteCounter, Markdown, Icon } from "@fider/components"
import { ResponseLozenge } from "@fider/components/post/ShowPostResponse"
import { heroiconsChatAlt2 as IconChatAlt2 } from "@fider/icons.generated"
import { HStack, VStack } from "@fider/components/layout"
import { getVotePosition } from "@fider/components/UserProfile/UserProfileSettings"
import { SponsorSpot, useSponsorPlacement } from "@fider/components/sponsorship/SponsorProvider"

export interface PendingPost {
  id: number
  height: number
  pending: true
}

export type PostListRow = Post | PendingPost

interface ListPostsProps {
  posts?: PostListRow[]
  tags: Tag[]
  emptyText: string
  loading?: boolean
  insertFeedAds?: boolean
  feedAdHeights?: Record<string, number>
}

interface PostWithTags {
  post: PostListRow
  tags: Tag[]
}

const PostSkeleton = ({ votePosition }: { votePosition: "left" | "right" }) => {
  const numTags = Math.floor(Math.random() * 3) + 2
  const tags = Array(numTags).fill(null)
  const isRight = votePosition === "right"

  return (
    <HStack spacing={4} align="start" className={`min-w-0 animate-pulse ${isRight ? "flex-row-reverse" : ""}`}>
      <div className="shrink-0">
        <VStack spacing={2} align="center">
          <div className="h-4 w-4 bg-surface-alt rounded"></div>
          <div className="h-8 w-8 bg-surface-alt rounded"></div>
          <div className="h-4 w-4 bg-surface-alt rounded"></div>
        </VStack>
      </div>
      <VStack className="flex-1 min-w-0" spacing={2}>
        <HStack spacing={0} className="gap-2 flex-wrap">
          {tags.map((_, i) => (
            <div key={i} className={`h-6 bg-surface-alt rounded ${i % 2 === 0 ? 'w-16' : 'w-20'}`}></div>
          ))}
        </HStack>
        <div className="h-6 w-3/4 bg-surface-alt rounded"></div>
        <div className="h-4 w-full bg-surface-alt rounded"></div>
        <div className="h-4 w-2/3 bg-surface-alt rounded"></div>
      </VStack>
    </HStack>
  )
}

const ListPostItem = (props: { post: Post; user?: CurrentUser; tags: Tag[]; votePosition: "left" | "right" }) => {
  const voteSection = (
    <div className="shrink-0">
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
    <VStack className="flex-1 min-w-0" spacing={2}>
      {props.tags.length >= 1 && (
        <HStack spacing={0} className="gap-2 flex-wrap">
          {props.tags.map((tag) => (
            <ShowTag key={tag.id} tag={tag} link />
          ))}
        </HStack>
      )}
      <a className="text-lg font-medium text-primary hover:text-primary-hover wrap-anywhere" href={`/posts/${props.post.number}/${props.post.slug}`} data-morph={`post-${props.post.number}`}>
        {props.post.title}
      </a>
      <div className="wrap-anywhere">
        <Markdown className="text-muted" maxLength={300} text={props.post.description} style="plainText" />
      </div>
      {props.post.status !== "open" && (
        <div className="mb-2 self-start">
          <ResponseLozenge status={props.post.status} response={props.post.response} small={true}/>
        </div>
      )}
    </VStack>
  )

  const isRight = props.votePosition === "right"

  return (
    <HStack spacing={4} align="start" className={`min-w-0 ${isRight ? "flex-row-reverse" : ""}`}>
      {voteSection}
      {contentSection}
    </HStack>
  )
}

export const ListPosts = (props: ListPostsProps) => {
  const feedPlacement = useSponsorPlacement("feed")
  const [votePosition, setVotePositionState] = useState<"left" | "right">("left")

  useEffect(() => {
    setVotePositionState(getVotePosition())
    
    const handleStorageChange = () => {
      setVotePositionState(getVotePosition())
    }
    window.addEventListener("storage", handleStorageChange)
    return () => window.removeEventListener("storage", handleStorageChange)
  }, [])

  const postsWithTags = useMemo((): PostWithTags[] => {
    if (!props.posts) return []
    return props.posts.map((post) => ({
      post,
      tags: "pending" in post ? [] : props.tags.filter((tag) => post.tags.includes(tag.slug)),
    }))
  }, [props.posts, props.tags])

  if (!props.posts && !props.loading) {
    return null
  }

  if (props.loading) {
    return (
      <VStack spacing={4} divide>
        {Array(15).fill(null).map((_, i) => (
          <PostSkeleton key={i} votePosition={votePosition} />
        ))}
      </VStack>
    )
  }

  if (postsWithTags.length === 0) {
    return <p className="text-center">{props.emptyText}</p>
  }

  const insertFeedAds = !!props.insertFeedAds

  return (
    <VStack spacing={4} divide>
      {postsWithTags.map(({ post, tags }, index) => {
        const every = feedPlacement?.every
        const showAd = insertFeedAds && every && (index + 1) % every === 0
        const feedIndex = every ? Math.floor((index + 1) / every) - 1 : 0
        const instanceId = `feed-${feedIndex}`
        return (
          <React.Fragment key={post.id}>
            <div
              data-post-id={post.id}
              data-post-pending={"pending" in post ? true : undefined}
              style={"pending" in post ? { height: post.height } : undefined}
            >
              {"pending" in post ? (
                <div className="h-full rounded-card bg-surface-alt" aria-label="Loading post" />
              ) : (
                <ListPostItem post={post} tags={tags} votePosition={votePosition} />
              )}
            </div>
            {showAd && (
              <SponsorSpot position="feed" instance={instanceId} reservedHeight={props.feedAdHeights?.[instanceId]} />
            )}
          </React.Fragment>
        )
      })}
    </VStack>
  )
}
