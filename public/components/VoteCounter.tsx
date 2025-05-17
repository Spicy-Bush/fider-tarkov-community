import "./VoteCounter.scss"

import React, { useState, useEffect } from "react"
import { Post, PostStatus, isPostLocked } from "@fider/models"
import { actions, classSet } from "@fider/services"
import { Icon, SignInModal } from "@fider/components"
import { useFider } from "@fider/hooks"
import FaCaretUp from "@fider/assets/images/fa-caretup.svg"
import FaCaretDown from "@fider/assets/images/fa-caretdown.svg"


export interface VoteCounterProps {
  post: Post
}

export const VoteCounter = (props: VoteCounterProps) => {
  const fider = useFider()
  // Initialize with the post's voteType value: -1 for down, 0 for none, 1 for up
  const [voteType, setVoteType] = useState<'up' | 'down' | 'none'>(
    props.post.voteType === 1 ? 'up' : props.post.voteType === -1 ? 'down' : 'none'
  )
  const [upvotes, setUpvotes] = useState(props.post.upvotes || 0)
  const [downvotes, setDownvotes] = useState(props.post.downvotes || 0)
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  
  // Calculate vote difference
  const votesDifference = upvotes - downvotes
  
  // Update local state when props change
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

    // If clicking the same button that is already active, remove the vote
    if (voteType === type) {
      response = await actions.removeVote(props.post.number);
      newVoteType = 'none';
      if (type === 'up') {
        upvoteChange = -1;
      } else {
        downvoteChange = -1;
      }
    } 
    // If clicking the opposite button, switch the vote type
    else if (voteType !== 'none') {
      response = await actions.toggleVote(props.post.number, type);
      newVoteType = type;
      if (type === 'up') {
        upvoteChange = 1;
        downvoteChange = -1;
      } else {
        upvoteChange = -1;
        downvoteChange = 1;
      }
    } 
    // If no vote currently, add the selected vote type
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
      setVoteType(newVoteType);
      setUpvotes(upvotes + upvoteChange);
      setDownvotes(downvotes + downvoteChange);
    }
  }

  const hideModal = () => setIsSignInModalOpen(false)

  const status = PostStatus.Get(props.post.status)
  const isDisabled = status.closed || fider.isReadOnly || isPostLocked(props.post)

  const upvoteClassName = classSet({
    "c-vote-counter__button": true,
    "c-vote-counter__button--voted": !status.closed && voteType === 'up',
    "c-vote-counter__button--disabled": isDisabled,
  })

  const downvoteClassName = classSet({
    "c-vote-counter__button": true, 
    "c-vote-counter__button--down": true,
    "c-vote-counter__button--voted": !status.closed && voteType === 'down',
    "c-vote-counter__button--disabled": isDisabled,
  })

  const countClassName = classSet({
    "c-vote-counter__count": true,
    "c-vote-counter__count--positive": votesDifference > 0,
    "c-vote-counter__count--negative": votesDifference < 0,
    "c-vote-counter__count--neutral": votesDifference === 0,
  })

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
      <div className="c-vote-counter">
        <button 
          className={upvoteClassName} 
          onClick={() => handleVote('up')}
          disabled={isDisabled}
        >
          <Icon sprite={FaCaretUp} height="16" width="16" />
        </button>
        
        <div className={countClassName}>
          {votesDifference}
        </div>
        
        <button 
          className={downvoteClassName} 
          onClick={() => handleVote('down')}
          disabled={isDisabled}
        >
          <Icon sprite={FaCaretDown} height="16" width="16" />
        </button>
      </div>
    </>
  )
}