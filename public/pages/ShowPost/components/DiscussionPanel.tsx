import React from "react"
import { Post } from "@fider/models"
import { Discussion } from "@fider/components/discussion/Discussion"
import { FollowButton } from "./FollowButton"

interface DiscussionPanelProps {
  post: Post
  subscribed: boolean
  onCommentAdded?: () => void
}

export function DiscussionPanel(props: DiscussionPanelProps) {
  return (
    <div className="mt-8">
      <Discussion
        owner={{
          kind: "post",
          id: props.post.id,
          number: props.post.number,
          title: props.post.title,
          url: `/posts/${props.post.number}/${props.post.slug}`,
        }}
        onCommentAdded={props.onCommentAdded}
        headerActions={<FollowButton post={props.post} subscribed={props.subscribed} />}
      />
    </div>
  )
}
