import React, { useMemo, useState, useEffect } from "react"
import { Post, Tag, CurrentUser } from "@fider/models"
import { ShowTag, VoteCounter, Markdown, Icon } from "@fider/components"
import { ResponseLozenge } from "@fider/components/post/ShowPostResponse"
import { heroiconsChatAlt2 as IconChatAlt2 } from "@fider/icons.generated"
import { HStack, VStack } from "@fider/components/layout"
import { getVotePosition } from "@fider/components/UserProfile/UserProfileSettings"
import { AdSlot, FeedNativeAd } from "@fider/components/sponsorship"
import { FEED_AD_EVERY, PublicAd } from "@fider/models"

interface ListPostsProps {
  posts?: Post[]
  tags: Tag[]
  emptyText: string
  loading?: boolean
  /** When true, insert feed_native AdSlots every FEED_AD_EVERY posts. Default off so SimilarPosts stays clean. */
  insertFeedAds?: boolean
  /** Page-owned selection map keyed by feed-{i}. Required when insertFeedAds. */
  feedAds?: Record<string, PublicAd | null | undefined>
  feedAdsLoaded?: boolean
  /** Select HTTP failure — do not treat as empty inventory / AdSense. */
  feedAdsError?: boolean
}

interface PostWithTags {
  post: Post
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
      <a className="text-lg font-medium text-primary hover:text-primary-hover wrap-anywhere" href={`/posts/${props.post.number}/${props.post.slug}`}>
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
      tags: props.tags.filter((tag) => post.tags.includes(tag.slug)),
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
        const showAd = insertFeedAds && (index + 1) % FEED_AD_EVERY === 0
        const feedIndex = Math.floor((index + 1) / FEED_AD_EVERY) - 1
        const instanceId = `feed-${feedIndex}`
        const selectFailed = !!props.feedAdsError
        const ad: PublicAd | null | undefined = showAd
          ? props.feedAdsLoaded && !selectFailed
            ? props.feedAds?.[instanceId] ?? null
            : undefined
          : undefined
        return (
          <React.Fragment key={post.id}>
            <ListPostItem post={post} tags={tags} votePosition={votePosition} />
            {showAd && (
              ad ? (
                <FeedNativeAd ad={ad} />
              ) : (
                <AdSlot
                  instanceId={instanceId}
                  placementId="feed_native"
                  ad={ad}
                  selectFailed={selectFailed}
                  placement={{ kind: "native" }}
                />
              )
            )}
          </React.Fragment>
        )
      })}
    </VStack>
  )
}
