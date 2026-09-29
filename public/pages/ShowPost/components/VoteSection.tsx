import React from "react"
import { Post } from "@fider/models"
import { classSet } from "@fider/services/utils"
import { usePostVote } from "@fider/hooks/usePostVote"
import { AnimatedCount } from "@fider/components/common/AnimatedCount"
import { Button } from "@fider/components/common/Button"
import { Icon } from "@fider/components/common/Icon"
import { SignInModal } from "@fider/components/auth/SignInModal"
import { heroiconsThumbsup as IconThumbsUp, heroiconsThumbsdown as IconThumbsDown } from "@fider/icons.generated"
import { Trans } from "@lingui/macro"
import { HStack, VStack } from "@fider/components/layout/Stack"

interface VoteSectionProps {
  post: Post
}

export const VoteSection = (props: VoteSectionProps) => {
  const { voteType, upvotes, downvotes, isDisabled, isSignInModalOpen, closeSignInModal, chooseVote } = usePostVote(props.post)

  const totalEngagement = upvotes + downvotes
  const votesDifference = upvotes - downvotes
  const upvotePercentage = totalEngagement > 0 ? (upvotes / totalEngagement) * 100 : 50

  const countClassName = classSet({
    "text-2xl font-bold min-w-10 text-center transition-colors duration-100": true,
    "text-success": votesDifference > 0,
    "text-danger": votesDifference < 0,
    "text-muted": votesDifference === 0,
  })

  return (
    <>
      <SignInModal isOpen={isSignInModalOpen} onClose={closeSignInModal} />
      <div className="w-full">
        <div className="flex items-center justify-between gap-4 w-full max-md:gap-2">
          <Button
            variant="secondary"
            onClick={() => chooseVote('up')}
            disabled={isDisabled}
            loading={false}
            className={classSet({
              "flex-1 overflow-hidden whitespace-nowrap text-ellipsis md:max-w-[30%] max-md:text-sm": true,
              "bg-success! text-white! border-success! vote-pop": voteType === 'up',
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
            <AnimatedCount value={votesDifference} className={countClassName} />
          </div>

          <Button
            variant="secondary"
            onClick={() => chooseVote('down')}
            disabled={isDisabled}
            loading={false}
            className={classSet({
              "flex-1 overflow-hidden whitespace-nowrap text-ellipsis md:max-w-[30%] max-md:text-sm": true,
              "bg-danger! text-white! border-danger! vote-pop": voteType === 'down',
              "text-danger!": voteType !== 'down',
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
                className="h-full bg-success transition-[width] duration-200 ease-out"
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
