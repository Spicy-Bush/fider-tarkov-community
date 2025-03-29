import React from "react"
import { Post, Tag, CurrentUser } from "@fider/models"
import { ShowTag, VoteCounter, Markdown, Icon, ResponseLozenge } from "@fider/components"
import IconChatAlt2 from "@fider/assets/images/heroicons-chat-alt-2.svg"
import { HStack, VStack } from "@fider/components/layout"

interface ListPostsProps {
  posts?: Post[]
  tags: Tag[]
  emptyText: string
}

const ListPostItem = (props: { post: Post; user?: CurrentUser; tags: Tag[] }) => {
  return (
    <HStack spacing={4} align="start" className="c-posts-container__post">
      <div>
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
      <VStack className="w-full" spacing={2}>
        {props.tags.length >= 1 && (
          <HStack spacing={0} className="gap-2 flex-wrap">
            {props.tags.map((tag) => (
              <ShowTag key={tag.id} tag={tag} link />
            ))}
          </HStack>
        )}
        <HStack justify="between">
          <a className="text-title hover:text-primary-base" href={`/posts/${props.post.number}/${props.post.slug}`}>
            {props.post.title}
          </a>
        </HStack>
        <Markdown className="text-gray-700" maxLength={300} text={props.post.description} style="plainText" />
        {props.post.status !== "open" && (
          <div className="mb-2 align-self-start">
            <ResponseLozenge status={props.post.status} response={props.post.response} small={true}/>
          </div>
        )}
      </VStack>
    </HStack>
  )
}

export const ListPosts = (props: ListPostsProps) => {
  if (!props.posts) {
    return null
  }

  if (props.posts.length === 0) {
    return <p className="text-center">{props.emptyText}</p>
  }

  return (
    <VStack spacing={4} divide>
      {props.posts.map((post) => (
        <ListPostItem key={post.id} post={post} tags={props.tags.filter((tag) => post.tags.indexOf(tag.slug) >= 0)} />
      ))}
    </VStack>
  )
}
