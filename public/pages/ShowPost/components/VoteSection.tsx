import "./VoteSection.scss"

import React, { useState } from "react"
import { Post, PostStatus, isPostLocked } from "@fider/models"
import { actions, classSet } from "@fider/services"
import { Button, Icon, SignInModal } from "@fider/components"
import { useFider } from "@fider/hooks"
import IconThumbsUp from "@fider/assets/images/heroicons-thumbsup.svg"
import IconThumbsDown from "@fider/assets/images/heroicons-thumbsdown.svg"
import { Trans } from "@lingui/macro"
import { HStack } from "@fider/components/layout"

interface VoteSectionProps {
  post: Post
  votes: number
}

export const VoteSection = (props: VoteSectionProps) => {
  const fider = useFider()
  const [voteType, setVoteType] = useState<'up' | 'down' | 'none'>(
    props.post.voteType === 1 ? 'up' : props.post.voteType === -1 ? 'down' : 'none'
  )
  const [votes, setVotes] = useState(props.votes)
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)

  const handleVote = async (type: 'up' | 'down') => {
    if (!fider.session.isAuthenticated) {
      setIsSignInModalOpen(true)
      return
    }

    const status = PostStatus.Get(props.post.status)
    if (status.closed || fider.isReadOnly) {
      return
    }

    let response;
    let newVoteType: 'up' | 'down' | 'none';
    let countChange = 0;

    if (voteType === type) {
      response = await actions.removeVote(props.post.number);
      newVoteType = 'none';
      countChange = type === 'up' ? -1 : 1;
    } 
    
    else if (voteType !== 'none') {
      response = await actions.toggleVote(props.post.number, type);
      newVoteType = type;
      countChange = type === 'up' ? 2 : -2;
    } 

    else {
      if (type === 'up') {
        response = await actions.addVote(props.post.number);
        newVoteType = 'up';
        countChange = 1;
      } else {
        response = await actions.addDownVote(props.post.number);
        newVoteType = 'down';
        countChange = -1;
      }
    }

    if (response.ok) {
      setVotes(votes + countChange);
      setVoteType(newVoteType);
    }
  }

  const hideModal = () => setIsSignInModalOpen(false)

  const status = PostStatus.Get(props.post.status)
  const isDisabled = status.closed || fider.isReadOnly || isPostLocked(props.post)

  const countClassName = classSet({
    "c-vote-section__count": true,
    "c-vote-section__count--positive": votes > 0 && voteType === 'up',
    "c-vote-section__count--negative": votes < 0 || voteType === 'down',
    "c-vote-section__count--neutral": votes === 0,
  })

  const upButtonClassName = classSet({
    "c-vote-section__button": true,
    "c-vote-section__button--up": true,
    "voted": voteType === 'up'
  })

  const downButtonClassName = classSet({
    "c-vote-section__button": true,
    "c-vote-section__button--down": true,
    "voted": voteType === 'down'
  })

  const voteLabel = votes === 1 ? "Vote" : "Votes"

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
      <div className="c-vote-section__buttons">
        <Button 
          variant={voteType === 'up' ? "primary" : "secondary"} 
          onClick={() => handleVote('up')} 
          disabled={isDisabled}
          className={upButtonClassName}
        >
          <HStack spacing={2} justify="center" className="w-full">
            <Icon sprite={IconThumbsUp} /> 
            <span>
              {voteType === 'up' ? 
                <Trans id="action.voted">Voted!</Trans> : 
                <Trans id="action.vote">Vote</Trans>}
            </span>
          </HStack>
        </Button>
        
        <div className="c-vote-section__count-wrapper">
          <span className={countClassName}>
            {votes}
          </span>
          <span className="text-semibold text-lg">{voteLabel}</span>
        </div>
        
        <Button 
          variant={voteType === 'down' ? "danger" : "secondary"} 
          onClick={() => handleVote('down')} 
          disabled={isDisabled}
          className={downButtonClassName}
        >
          <HStack spacing={2} justify="center" className="w-full">
            <Icon sprite={IconThumbsDown} /> 
            <span>
              <Trans id="action.downvote">Downvote</Trans>
            </span>
          </HStack>
        </Button>
      </div>
    </>
  )
}
