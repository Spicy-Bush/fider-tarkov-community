import React, { useState } from "react"
import { Post, Vote } from "@fider/models"
import { AvatarStack } from "@fider/components/common/AvatarStack"
import { Button } from "@fider/components/common/Button"
import { VotesModal } from "./VotesModal"
import { VStack } from "@fider/components/layout/Stack"
import { Trans } from "@lingui/react/macro"

interface VotesPanelProps {
  post: Post
  hideTitle?: boolean
  votes: Vote[]
}

export const VotesPanel = (props: VotesPanelProps) => {
  const [isVotesModalOpen, setIsVotesModalOpen] = useState(false)
  const canShowAll = props.post.permissions.viewVotes
  const hasVotes = props.votes.length > 0

  const openModal = () => {
    if (canShowAll) {
      setIsVotesModalOpen(true)
    }
  }

  const closeModal = () => setIsVotesModalOpen(false)
  
  return (
    <VStack spacing={4}>
      <VotesModal post={props.post} isOpen={isVotesModalOpen} onClose={closeModal} />
      {!props.hideTitle && (
        <span className="text-category">
          <Trans id="label.voters">Voters</Trans>
        </span>
      )}
      {hasVotes ? (
        <>
          <div className="flex">
            <AvatarStack users={props.votes.slice(0, 8).map((x) => x.user)} overlap={true} />
          </div>
          {canShowAll && (
            <Button variant="tertiary" size="small" onClick={openModal}>
                View Details
            </Button>
          )}
        </>
      ) : (
        <span className="text-muted">
          <Trans id="label.none">None</Trans>
        </span>
      )}
    </VStack>
  )
}
