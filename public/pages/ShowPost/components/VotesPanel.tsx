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
  const canShowAll = fider.session.isAuthenticated && Fider.session.user.isCollaborator
  // using negative here because even if the vote it negative itself, it's still a 'vote', 
  // and as such, we need to show the total number of said votes.
  const totalVotes = Math.abs(props.post.votesCount)

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
      {totalVotes > 0 ? (
        <>
          {props.votes.length > 0 && (
            <HStack spacing={0} className="gap-2">
              <AvatarStack users={props.votes.map((x) => x.user)} overlap={false} />
            </HStack>
          )}
          {totalVotes > 1 ? (
            <Button variant="tertiary" size="small" disabled={!canShowAll} onClick={openModal}>
              {totalVotes} votes
            </Button>
          ) : (
            <span>{totalVotes} vote</span>
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
