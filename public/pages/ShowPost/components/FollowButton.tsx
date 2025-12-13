import React, { useState } from "react"
import { Button, Icon } from "@fider/components"
import { actions } from "@fider/services"
import { useFider } from "@fider/hooks"
import { heroiconsPlus as IconPlus, heroiconsCheck as IconCheck } from "@fider/icons.generated"
import { VStack } from "@fider/components/layout"
import { Trans } from "@lingui/macro"
import { Post, isPostLocked } from "@fider/models"

export interface NotificationsPanelProps {
  post: Post
  subscribed: boolean
}

export const FollowButton = (props: NotificationsPanelProps) => {
  const fider = useFider()
  const [subscribed, setSubscribed] = useState(props.subscribed)
  const isLocked = isPostLocked(props.post) || fider.isReadOnly

  const subscribeOrUnsubscribe = async () => {
    const action = subscribed ? actions.unsubscribe : actions.subscribe

    const response = await action(props.post.number)
    if (response.ok) {
      setSubscribed(!subscribed)
    }
  }

  if (!fider.session.isAuthenticated) {
    return null
  }

  const button = subscribed ? (
    <Button 
      variant="primary"
      className="w-full no-focus" 
      onClick={subscribeOrUnsubscribe} 
      disabled={isLocked}
    >
      <Icon sprite={IconCheck} />{" "}
      <span>
        <Trans id="label.following">Following</Trans>
      </span>
    </Button>
  ) : (
    <Button 
      variant="secondary"
      className="w-full no-focus" 
      onClick={subscribeOrUnsubscribe} 
      disabled={isLocked}
    >
      <Icon sprite={IconPlus} />
      <span>
        <Trans id="label.follow">Follow</Trans>
      </span>
    </Button>
  )

  return <VStack>{button}</VStack>
}