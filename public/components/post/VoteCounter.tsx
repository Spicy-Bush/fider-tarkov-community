import React, { useState, useEffect } from "react"
import { Post, PostStatus, isPostLocked } from "@fider/models"
import { actions, classSet } from "@fider/services"
import { Icon, SignInModal } from "@fider/components"
import { useFider } from "@fider/hooks"
import { faCaretup as FaCaretUp, faCaretdown as FaCaretDown } from "@fider/icons.generated"


export interface VoteCounterProps {
  post: Post
}

export const VoteCounter = (props: VoteCounterProps) => {
  const fider = useFider()
  const [voteType, setVoteType] = useState<'up' | 'down' | 'none'>(
    props.post.voteType === 1 ? 'up' : props.post.voteType === -1 ? 'down' : 'none'
  )
  const [upvotes, setUpvotes] = useState(props.post.upvotes || 0)
  const [downvotes, setDownvotes] = useState(props.post.downvotes || 0)
  const [isSignInModalOpen, setIsSignInModalOpen] = useState(false)
  
  const votesDifference = upvotes - downvotes
  
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
      if (type === 'up') {
        upvoteChange = -1;
      } else {
        downvoteChange = -1;
      }
    } 
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
    "text-lg w-11 font-bold cursor-pointer text-center mx-auto py-0.5 pb-2 text-muted flex flex-col items-center [&_svg]:text-border-strong [&_svg]:-mb-0.5": true,
    "hover:text-success hover:[&_svg]:text-success": !isDisabled,
    "text-success [&_svg]:text-success": !status.closed && voteType === 'up',
    "opacity-50 cursor-not-allowed pointer-events-none": isDisabled,
  })

  const downvoteClassName = classSet({
    "text-lg w-11 font-bold cursor-pointer text-center mx-auto py-0.5 pb-2 text-muted flex flex-col items-center [&_svg]:text-border-strong [&_svg]:-mb-0.5": true,
    "hover:text-danger hover:[&_svg]:text-danger": !isDisabled,
    "text-danger [&_svg]:text-danger": !status.closed && voteType === 'down',
    "opacity-50 cursor-not-allowed pointer-events-none": isDisabled,
  })

  const countClassName = classSet({
    "font-bold": true,
    "text-foreground": votesDifference > 0,
    "text-danger": votesDifference < 0,
    "text-muted": votesDifference === 0,
  })

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
      <div className="flex flex-col items-center">
        <button 
          className={upvoteClassName} 
          onClick={() => handleVote('up')}
          disabled={isDisabled}
          aria-label="Upvote"
          aria-pressed={voteType === 'up'}
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
          aria-label="Downvote"
          aria-pressed={voteType === 'down'}
        >
          <Icon sprite={FaCaretDown} height="16" width="16" />
        </button>
      </div>
    </>
  )
}
