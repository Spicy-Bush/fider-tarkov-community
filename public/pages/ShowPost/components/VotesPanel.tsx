import React, { useState } from "react"
import { Post, Vote } from "@fider/models"
import { AvatarStack, Button } from "@fider/components"
import { Fider } from "@fider/services"
import { useFider } from "@fider/hooks"
import { VotesModal } from "./VotesModal"
import { HStack, VStack } from "@fider/components/layout"
import { Trans } from "@lingui/react/macro"

interface VotesPanelProps {
  post: Post
  hideTitle?: boolean
  votes: Vote[]
}

export const VotesPanel = (props: VotesPanelProps) => {
  const fider = useFider()
  const [isVotesModalOpen, setIsVotesModalOpen] = useState(false)
  const canShowAll = fider.session.isAuthenticated && (Fider.session.user.isCollaborator || Fider.session.user.isModerator || Fider.session.user.isAdministrator)
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
