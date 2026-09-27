import React from "react"
import { Post } from "@fider/models"
import { classSet } from "@fider/services"
import { usePostVote } from "@fider/hooks/usePostVote"
import { AnimatedCount, Icon, SignInModal } from "@fider/components"
import { faCaretup as FaCaretUp, faCaretdown as FaCaretDown } from "@fider/icons.generated"


export interface VoteCounterProps {
  post: Post
}

export const VoteCounter = (props: VoteCounterProps) => {
  const { voteType, upvotes, downvotes, status, isDisabled, isSignInModalOpen, closeSignInModal, chooseVote } = usePostVote(props.post)
  
  const votesDifference = upvotes - downvotes
  
  const upvoteClassName = classSet({
    "text-lg w-11 font-bold cursor-pointer text-center mx-auto py-0.5 pb-2 text-muted flex flex-col items-center [&_svg]:text-border-strong [&_svg]:-mb-0.5 [&_svg]:transition-[color,transform] [&_svg]:duration-100 active:[&_svg]:scale-90": true,
    "hover:text-success hover:[&_svg]:text-success": !isDisabled,
    "text-success [&_svg]:text-success vote-pop": !status.closed && voteType === 'up',
    "opacity-50 cursor-not-allowed pointer-events-none": isDisabled,
  })

  const downvoteClassName = classSet({
    "text-lg w-11 font-bold cursor-pointer text-center mx-auto py-0.5 pb-2 text-muted flex flex-col items-center [&_svg]:text-border-strong [&_svg]:-mb-0.5 [&_svg]:transition-[color,transform] [&_svg]:duration-100 active:[&_svg]:scale-90": true,
    "hover:text-danger hover:[&_svg]:text-danger": !isDisabled,
    "text-danger [&_svg]:text-danger vote-pop": !status.closed && voteType === 'down',
    "opacity-50 cursor-not-allowed pointer-events-none": isDisabled,
  })

  const countClassName = classSet({
    "font-bold transition-colors duration-100": true,
    "text-foreground": votesDifference > 0,
    "text-danger": votesDifference < 0,
    "text-muted": votesDifference === 0,
  })

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={closeSignInModal} />
      <div className="flex flex-col items-center">
        <button 
          className={upvoteClassName} 
          onClick={() => chooseVote('up')}
          disabled={isDisabled}
          aria-label="Upvote"
          aria-pressed={voteType === 'up'}
        >
          <Icon sprite={FaCaretUp} height="16" width="16" />
        </button>
        
        <AnimatedCount value={votesDifference} className={countClassName} />
        
        <button 
          className={downvoteClassName} 
          onClick={() => chooseVote('down')}
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
