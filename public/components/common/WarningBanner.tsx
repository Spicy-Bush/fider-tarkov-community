import React from "react"
import { useFider } from "@fider/hooks"
import { Trans } from "@lingui/react/macro"
import { Icon } from "./Icon"
import IconExclamation from "@fider/assets/images/heroicons-exclamation.svg"
import IconVolumeOff from "@fider/assets/images/heroicons-volume-off.svg"
import { Button } from "./Button"
import { classSet } from "@fider/services"
import "./WarningBanner.scss"

export const WarningBanner = () => {
  const fider = useFider()
  const [isVisible, setIsVisible] = React.useState(true)

  if (!fider.session.isAuthenticated) {
    return null
  }

  const { hasWarning, isMuted } = fider.session.user

  if (!hasWarning && !isMuted) {
    return null
  }

  React.useEffect(() => {
    const lastDismissed = localStorage.getItem('lastWarningDismissed')
    if (lastDismissed) {
      const dismissedTime = new Date(lastDismissed).getTime()
      const now = new Date().getTime()
      const hoursSinceDismissed = (now - dismissedTime) / (1000 * 60 * 60)
      setIsVisible(hoursSinceDismissed >= 24)
    }
  }, [hasWarning, isMuted])

  const handleDismiss = () => {
    setIsVisible(false)
    localStorage.setItem('lastWarningDismissed', new Date().toISOString())
  }

  const className = classSet({
    "c-warning-banner": true,
    "c-warning-banner--warning": hasWarning,
    "c-warning-banner--muted": isMuted,
  })

  if (!isVisible) {
    return null
  }

  return (
    <div className={className}>
      <div className="c-warning-banner__content">
        <Icon sprite={hasWarning ? IconExclamation : IconVolumeOff} className="h-5" />
        <div className="c-warning-banner__message">
          {hasWarning && (
            <Trans id="warning.banner.message">
              You have been warned. <a href="/profile#standing"><Trans id="warning.banner.review">Please review your recent behavior and ensure it aligns with our community guidelines.</Trans></a>
            </Trans>
          )}
          {isMuted && (
            <Trans id="warning.banner.muted">
              You have been muted. You will not be able to post or comment until the mute expires.
            </Trans>
          )}
        </div>
        <Button variant="tertiary" size="small" onClick={handleDismiss}>
          <Trans id="action.dismiss">Dismiss</Trans>
        </Button>
      </div>
    </div>
  )
} 