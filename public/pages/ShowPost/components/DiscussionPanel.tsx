import React, { useState } from "react"
import { CurrentUser, Comment, Post, ReportReason } from "@fider/models"
import { ShowComment } from "./ShowComment"
import { CommentInput } from "./CommentInput"
import { HStack, VStack } from "@fider/components/layout"
import { Trans } from "@lingui/react/macro"
import { FollowButton } from "./FollowButton"

interface DiscussionPanelProps {
  user?: CurrentUser
  post: Post
  comments: Comment[]
  highlightedComment?: number
  subscribed: boolean
  reportedCommentIds: number[]
  dailyLimitReached: boolean
  reportReasons?: ReportReason[]
  onCommentAdded?: () => void
}

export const DiscussionPanel = (props: DiscussionPanelProps) => {
  const [comments, setComments] = useState<Comment[]>(props.comments)

  const handleCommentAdded = (newComment: Comment) => {
    setComments(prev => [...prev, newComment])
    if (props.onCommentAdded) {
      props.onCommentAdded()
    }
  }

  return (
    <>
      <VStack spacing={2} className="c-comment-list mt-8">
        <HStack justify="between" align="center">
          <span className="text-category">
            <Trans id="label.discussion">Discussion</Trans>
          </span>
          <FollowButton post={props.post} subscribed={props.subscribed} />
        </HStack>
        <VStack spacing={4} className="c-comment-list">
          {comments.map((c) => (
            <ShowComment
              key={c.id}
              post={props.post}
              comment={c}
              highlighted={props.highlightedComment === c.id}
              hasReported={props.reportedCommentIds.includes(c.id)}
              dailyLimitReached={props.dailyLimitReached}
              reportReasons={props.reportReasons}
            />
          ))}
          <CommentInput post={props.post} onCommentAdded={handleCommentAdded} />
        </VStack>
      </VStack>
    </>
  )
}
