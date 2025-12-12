import "./VoteSection.scss"

import React from "react"
import { Post } from "@fider/models"
import { classSet } from "@fider/services"
import { HStack, VStack } from "@fider/components/layout"

interface VoteSectionCompactProps {
  post: Post
}

export const VoteSectionCompact = (props: VoteSectionCompactProps) => {
  const upvotes = props.post.upvotes || 0
  const downvotes = props.post.downvotes || 0
  const totalEngagement = upvotes + downvotes
  const votesDifference = upvotes - downvotes
  const upvotePercentage = totalEngagement > 0 ? (upvotes / totalEngagement) * 100 : 50

  const countClassName = classSet({
    "c-vote-section__count": true,
    "c-vote-section__count--positive": votesDifference > 0,
    "c-vote-section__count--negative": votesDifference < 0,
    "c-vote-section__count--neutral": votesDifference === 0,
  })

  return (
    <div className="c-vote-section c-vote-section--compact w-full">
      <HStack spacing={2} align="center" justify="center">
        <span className={countClassName}>{votesDifference}</span>
      </HStack>
      
      {totalEngagement > 10 && (
        <VStack spacing={1} className="w-full mt-2">
          <div className="c-vote-section__bar">
            <div 
              className="c-vote-section__bar-upvotes" 
              style={{ width: `${upvotePercentage}%` }}
            />
          </div>
        </VStack>
      )}
    </div>
  )
}

