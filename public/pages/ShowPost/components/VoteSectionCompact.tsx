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
    "text-2xl font-bold min-w-10 text-center": true,
    "text-success": votesDifference > 0,
    "text-danger": votesDifference < 0,
    "text-muted": votesDifference === 0,
  })

  return (
    <div className="w-full">
      <HStack spacing={2} align="center" justify="center">
        <span className={countClassName}>{votesDifference}</span>
      </HStack>
      
      {totalEngagement > 10 && (
        <VStack spacing={1} className="w-full mt-2">
          <div className="h-1 bg-danger w-full rounded-badge overflow-hidden">
            <div 
              className="h-full bg-success" 
              style={{ width: `${upvotePercentage}%` }}
            />
          </div>
        </VStack>
      )}
    </div>
  )
}
