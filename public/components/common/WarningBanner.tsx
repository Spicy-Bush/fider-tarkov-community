import React, { useState, useEffect } from "react"
import { useFider } from "@fider/hooks"
import { Trans } from "@lingui/react/macro"
import { Icon } from "./Icon"
import { heroiconsExclamation as IconExclamation, heroiconsVolumeOff as IconVolumeOff, heroiconsX as IconX } from "@fider/icons.generated"
import { HStack } from "@fider/components/layout"
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

interface AlertBarProps {
  type: "warning" | "muted"
  onDismiss: () => void
  children: React.ReactNode
}

const AlertBar: React.FC<AlertBarProps> = ({ type, onDismiss, children }) => {
  const icon = type === "warning" ? IconExclamation : IconVolumeOff

  return (
    <div className={`c-warning-banner c-warning-banner--${type}`}>
      <HStack className="c-warning-banner__content" spacing={4} justify="between" align="center">
        <HStack spacing={4} align="center" className="c-warning-banner__left">
          <Icon sprite={icon} className="h-5" />
          <span className="c-warning-banner__message">{children}</span>
        </HStack>
        <Icon sprite={IconX} className="c-warning-banner__close h-5" onClick={onDismiss} />
      </HStack>
    </div>
  )
}

export const WarningBanner = () => {
  const fider = useFider()

  const isAuthenticated = fider.session.isAuthenticated
  const hasWarning = isAuthenticated ? fider.session.user.hasWarning : false
  const isMuted = isAuthenticated ? fider.session.user.isMuted : false
  const latestWarningId = isAuthenticated ? fider.session.user.latestWarningId : undefined
  const latestMuteId = isAuthenticated ? fider.session.user.latestMuteId : undefined

  const [warningVisible, setWarningVisible] = useState(() => 
    hasWarning && shouldShowForId(latestWarningId, 'dismissedWarning')
  )
  const [muteVisible, setMuteVisible] = useState(() => 
    isMuted && shouldShowForId(latestMuteId, 'dismissedMute')
  )

  useEffect(() => {
    if (hasWarning && latestWarningId) {
      setWarningVisible(shouldShowForId(latestWarningId, 'dismissedWarning'))
    }
  }, [hasWarning, latestWarningId])

  useEffect(() => {
    if (isMuted && latestMuteId) {
      setMuteVisible(shouldShowForId(latestMuteId, 'dismissedMute'))
    }
  }, [isMuted, latestMuteId])

  const handleDismissWarning = () => {
    if (latestWarningId) {
      setDismissalRecord('dismissedWarning', latestWarningId)
    }
    setWarningVisible(false)
  }

  const handleDismissMute = () => {
    if (latestMuteId) {
      setDismissalRecord('dismissedMute', latestMuteId)
    }
    setMuteVisible(false)
  }

  if (!isAuthenticated) {
    return null
  }

  const showWarning = warningVisible && hasWarning
  const showMute = muteVisible && isMuted

  if (!showWarning && !showMute) {
    return null
  }

  return (
    <div className="c-warning-banner-container">
      {showMute && (
        <AlertBar type="muted" onDismiss={handleDismissMute}>
          <Trans id="warning.banner.muted">
            You have been muted. You will not be able to post or comment until the mute expires. <a href="/profile#standing"><Trans id="warning.banner.review">Please review your recent behavior and ensure it aligns with our community guidelines.</Trans></a>
          </Trans>
        </AlertBar>
      )}
      {showWarning && (
        <AlertBar type="warning" onDismiss={handleDismissWarning}>
          <Trans id="warning.banner.message">
            You have been warned. <a href="/profile#standing"><Trans id="warning.banner.review">Please review your recent behavior and ensure it aligns with our community guidelines.</Trans></a>
          </Trans>
        </AlertBar>
      )}
    </div>
  )
}
