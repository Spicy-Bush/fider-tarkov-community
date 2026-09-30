import React, { useEffect, useState } from "react"
import { Post, Vote } from "@fider/models"
import { AvatarStack } from "@fider/components/common/AvatarStack"
import { Button } from "@fider/components/common/Button"
import { VotesModal } from "./VotesModal"
import { VStack } from "@fider/components/layout/Stack"
import { Trans } from "@lingui/react/macro"
import { listVotes } from "@fider/services/actions/post"
import { RequestError } from "@fider/services/http"
import * as notify from "@fider/services/notify"

interface VotesPanelProps {
  post: Post
  hideTitle?: boolean
  votes: Vote[]
  revision: number
}

export const VotesPanel = (props: VotesPanelProps) => {
  const [isVotesModalOpen, setIsVotesModalOpen] = useState(false)
  const [votes, setVotes] = useState(props.votes)
  const canShowAll = props.post.permissions.viewVotes
  const hasVotes = votes.length > 0

  useEffect(() => {
    if (props.revision === props.post.voteRevision) {
      setVotes(props.votes)
      return
    }

    const request = new AbortController()
    const refresh = async () => {
      try {
        const result = await listVotes(props.post.number, { preview: true, signal: request.signal })

        if (!request.signal.aborted && result.ok) {
          setVotes(result.data)
        }
      } catch (cause) {
        if (!request.signal.aborted) {
          if (!(cause instanceof RequestError)) throw cause

          notify.error("Could not refresh the voters. Please reload the page.")
        }
      }
    }

    void refresh()
    return () => request.abort()
  }, [props.post.number, props.post.voteRevision, props.revision, props.votes])

  const openModal = () => {
    if (canShowAll) {
      setIsVotesModalOpen(true)
    }
  }

  const closeModal = () => setIsVotesModalOpen(false)
  
  return (
    <VStack spacing={4}>
      <VotesModal post={props.post} revision={props.revision} isOpen={isVotesModalOpen} onClose={closeModal} />
      {!props.hideTitle && (
        <span className="text-category">
          <Trans id="label.voters">Voters</Trans>
        </span>
      )}
      {hasVotes ? (
        <>
          <div className="flex">
            <AvatarStack users={votes.map((x) => x.user)} overlap={true} />
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
