import "./VoteCounter.scss"

import React, { useState } from "react"
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
  const [votesCount, setVotesCount] = useState(props.post.votesCount)
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

    // If clicking the same button that is already active, remove the vote
    if (voteType === type) {
      response = await actions.removeVote(props.post.number);
      newVoteType = 'none';
      countChange = type === 'up' ? -1 : 1; // When removing upvote, decrease count; when removing downvote, increase count
    } 
    // If clicking the opposite button, switch the vote type
    else if (voteType !== 'none') {
      response = await actions.toggleVote(props.post.number, type);
      newVoteType = type;
      countChange = type === 'up' ? 2 : -2; // +2 for down -> up, -2 for up -> down
    } 
    // If no vote currently, add the selected vote type
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
      setVotesCount(votesCount + countChange);
      setVoteType(newVoteType);
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
    "c-vote-counter__count--positive": votesCount > 0,
    "c-vote-counter__count--negative": votesCount < 0,
    "c-vote-counter__count--neutral": votesCount === 0,
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
          {votesCount}
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