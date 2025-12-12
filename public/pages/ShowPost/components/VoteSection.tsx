import "./VoteSection.scss"

import React, { useState, useEffect } from "react"
import { Post, PostStatus, isPostLocked, isPostArchived } from "@fider/models"
import { actions, classSet } from "@fider/services"
import { Button, Icon, SignInModal } from "@fider/components"
import { useFider } from "@fider/hooks"
import { heroiconsThumbsup as IconThumbsUp, heroiconsThumbsdown as IconThumbsDown } from "@fider/icons.generated"
import { Trans } from "@lingui/macro"
import { HStack, VStack } from "@fider/components/layout"

interface VoteSectionProps {
  post: Post
}

export const VoteSection = (props: VoteSectionProps) => {
  const fider = useFider()
  const [voteType, setVoteType] = useState<'up' | 'down' | 'none'>(
    props.post.voteType === 1 ? 'up' : props.post.voteType === -1 ? 'down' : 'none'
  )
  const [upvotes, setUpvotes] = useState(props.post.upvotes || 0)
  const [downvotes, setDownvotes] = useState(props.post.downvotes || 0)
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  
  const totalEngagement = upvotes + downvotes
  const votesDifference = upvotes - downvotes
  const upvotePercentage = totalEngagement > 0 ? (upvotes / totalEngagement) * 100 : 50

  useEffect(() => {
    setUpvotes(props.post.upvotes || 0)
    setDownvotes(props.post.downvotes || 0)
  }, [props.post.upvotes, props.post.downvotes])

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
    let upvoteChange = 0;
    let downvoteChange = 0;

    if (voteType === type) {
      response = await actions.removeVote(props.post.number);
      newVoteType = 'none';
      
      // Remove upvote or downvote
      if (type === 'up') {
        upvoteChange = -1;
      } else {
        downvoteChange = -1;
      }
    } 
    
    else if (voteType !== 'none') {
      response = await actions.toggleVote(props.post.number, type);
      newVoteType = type;
      
      // Switch vote type
      if (type === 'up') {
        upvoteChange = 1;
        downvoteChange = -1;
      } else {
        upvoteChange = -1;
        downvoteChange = 1;
      }
    } 

    else {
      if (type === 'up') {
        response = await actions.addVote(props.post.number);
        newVoteType = 'up';
        upvoteChange = 1;
      } else {
        response = await actions.addDownVote(props.post.number);
        newVoteType = 'down';
        downvoteChange = 1;
      }
    }

    if (response.ok) {
      if (isPostArchived(props.post)) {
        location.reload() // change this later to be like, idk a replace in place or something
        return
      }
      setVoteType(newVoteType);
      setUpvotes(upvotes + upvoteChange);
      setDownvotes(downvotes + downvoteChange);
    }
  }

  const hideModal = () => setIsSignInModalOpen(false)

  const status = PostStatus.Get(props.post.status)
  const isDisabled = status.closed || fider.isReadOnly || isPostLocked(props.post)

  const countClassName = classSet({
    "c-vote-section__count": true,
    "c-vote-section__count--positive": votesDifference > 0,
    "c-vote-section__count--negative": votesDifference < 0,
    "c-vote-section__count--neutral": votesDifference === 0,
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

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
      <div className="c-vote-section w-full">
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
                <Trans id="action.upvote">Upvote</Trans>
              </span>
            </HStack>
          </Button>
          
          <div className="c-vote-section__count-wrapper">
            <span className={countClassName}>
              {votesDifference}
            </span>
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
        
        {totalEngagement > 10 && (
          <VStack spacing={1} className="w-full">
            <div className="c-vote-section__bar">
              <div 
                className="c-vote-section__bar-upvotes" 
                style={{ width: `${upvotePercentage}%` }}
              />
            </div>
            <div className="c-vote-section__engagement">
              <span className="text-xs text-gray-500">
                <Trans id="votes.engagement">{totalEngagement} total votes</Trans>
              </span>
            </div>
          </VStack>
        )}
      </div>
    </>
  )
}
