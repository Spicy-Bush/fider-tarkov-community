import React from "react"
import { useFider } from "@fider/hooks"
import { Trans } from "@lingui/react/macro"
import { Icon } from "./Icon"
import { heroiconsExclamation as IconExclamation, heroiconsVolumeOff as IconVolumeOff } from "@fider/icons.generated"
import { Button } from "./Button"
import { classSet } from "@fider/services"
import "./WarningBanner.scss"

interface DismissalRecord {
  id: number
  timestamp: string
}

const DISMISSAL_DURATION_HOURS = 24

const getDismissalRecord = (key: string): DismissalRecord | null => {
  try {
    const stored = localStorage.getItem(key)
    if (stored) {
      return JSON.parse(stored)
    }
  } catch {
    localStorage.removeItem(key)
  }
  return null
}

const setDismissalRecord = (key: string, id: number): void => {
  const record: DismissalRecord = {
    id,
    timestamp: new Date().toISOString()
  }
  localStorage.setItem(key, JSON.stringify(record))
}

const shouldShowForId = (id: number | undefined, dismissalKey: string): boolean => {
  if (!id || id === 0) return false
  
  const record = getDismissalRecord(dismissalKey)
  if (!record) return true
  
  if (record.id !== id) return true
  
  const dismissedTime = new Date(record.timestamp).getTime()
  const now = new Date().getTime()
  const hoursSinceDismissed = (now - dismissedTime) / (1000 * 60 * 60)
  
  return hoursSinceDismissed >= DISMISSAL_DURATION_HOURS
}

export const WarningBanner = () => {
  const fider = useFider()
  const [isVisible, setIsVisible] = React.useState(true)

  const isAuthenticated = fider.session.isAuthenticated
  const hasWarning = isAuthenticated ? fider.session.user.hasWarning : false
  const isMuted = isAuthenticated ? fider.session.user.isMuted : false
  const latestWarningId = isAuthenticated ? fider.session.user.latestWarningId : undefined
  const latestMuteId = isAuthenticated ? fider.session.user.latestMuteId : undefined

  const showWarning = hasWarning && shouldShowForId(latestWarningId, 'dismissedWarning')
  const showMute = isMuted && shouldShowForId(latestMuteId, 'dismissedMute')

  React.useEffect(() => {
    setIsVisible(true)
  }, [latestWarningId, latestMuteId])

  const handleDismiss = () => {
    setIsVisible(false)
    if (showWarning && latestWarningId) {
      setDismissalRecord('dismissedWarning', latestWarningId)
    }
    if (showMute && latestMuteId) {
      setDismissalRecord('dismissedMute', latestMuteId)
    }
  }

  const className = classSet({
    "c-warning-banner": true,
    "c-warning-banner--warning": showWarning && !showMute,
    "c-warning-banner--muted": showMute,
  })

  if (!isAuthenticated || (!showWarning && !showMute) || !isVisible) {
    return null
  }

  return (
    <div className={className}>
      <div className="c-warning-banner__content">
        <Icon sprite={showWarning ? IconExclamation : IconVolumeOff} className="h-5" />
        <div className="c-warning-banner__message">
          {showWarning && (
            <Trans id="warning.banner.message">
              You have been warned. <a href="/profile#standing"><Trans id="warning.banner.review">Please review your recent behavior and ensure it aligns with our community guidelines.</Trans></a>
            </Trans>
          )}
          {showMute && (
            <Trans id="warning.banner.muted">
              You have been muted. You will not be able to post or comment until the mute expires. <a href="/profile#standing"><Trans id="warning.banner.review">Please review your recent behavior and ensure it aligns with our community guidelines.</Trans></a>
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