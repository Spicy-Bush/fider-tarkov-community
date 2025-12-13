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
  onVoteChange?: (upvotes: number, downvotes: number) => void
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
      if (isPostArchived(props.post)) {
        location.reload()
        return
      }
      setVoteType(newVoteType);
      const newUpvotes = upvotes + upvoteChange;
      const newDownvotes = downvotes + downvoteChange;
      setUpvotes(newUpvotes);
      setDownvotes(newDownvotes);
      if (props.onVoteChange) {
        props.onVoteChange(newUpvotes, newDownvotes);
      }
    }
  }

  const hideModal = () => setIsSignInModalOpen(false)

  const status = PostStatus.Get(props.post.status)
  const isDisabled = status.closed || fider.isReadOnly || isPostLocked(props.post)

  const countClassName = classSet({
    "text-2xl font-bold min-w-10 text-center": true,
    "text-success": votesDifference > 0,
    "text-danger": votesDifference < 0,
    "text-muted": votesDifference === 0,
  })

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={hideModal} />
      <div className="w-full">
        <div className="flex items-center justify-between gap-4 w-full max-md:gap-2">
          <Button 
            variant="secondary"
            onClick={() => handleVote('up')} 
            disabled={isDisabled}
            className={classSet({
              "flex-1 overflow-hidden whitespace-nowrap text-ellipsis md:max-w-[30%] max-md:text-sm": true,
              "!bg-success !text-white !border-success": voteType === 'up',
              "text-success": voteType !== 'up',
            })}
          >
            <HStack spacing={2} justify="center" className="w-full">
              <Icon sprite={IconThumbsUp} /> 
              <span>
                <Trans id="action.upvote">Upvote</Trans>
              </span>
            </HStack>
          </Button>
          
          <div className="flex items-center justify-center min-w-10 text-center max-md:min-w-8">
            <span className={countClassName}>
              {votesDifference}
            </span>
          </div>
          
          <Button 
            variant="secondary"
            onClick={() => handleVote('down')} 
            disabled={isDisabled}
            className={classSet({
              "flex-1 overflow-hidden whitespace-nowrap text-ellipsis md:max-w-[30%] max-md:text-sm": true,
              "!bg-danger !text-white !border-danger": voteType === 'down',
              "!text-danger": voteType !== 'down',
            })}
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
            <div className="h-1 bg-danger mt-2 w-full rounded-badge overflow-hidden">
              <div 
                className="h-full bg-success" 
                style={{ width: `${upvotePercentage}%` }}
              />
            </div>
            <div className="flex justify-center w-full mt-1 italic">
              <span className="text-xs text-muted">
                <Trans id="votes.engagement">{totalEngagement} total votes</Trans>
              </span>
            </div>
          </VStack>
        )}
      </div>
    </>
  )
}
