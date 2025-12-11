import React, { useState } from "react"
import { useUserProfile } from "./context"
import { Trans } from "@lingui/react/macro"
import { actions, Failure } from "@fider/services"
import { UserStatus } from "@fider/models"
import { ModerationModal } from "@fider/components"

export const UserProfileActions: React.FC = () => {
  const { user, canModerate, canBlock, refreshStanding, refreshUser } = useUserProfile()

  const [moderationModal, setModerationModal] = useState({
    isOpen: false,
    actionType: "mute" as "mute" | "warning",
    error: undefined as Failure | undefined,
  })

  if (!user) return null

  const isBlocked = user.status === UserStatus.Blocked || user.status === 2

  const openMuteModal = () => {
    setModerationModal({
      isOpen: true,
      actionType: "mute",
      error: undefined,
    })
  }

  const openWarnModal = () => {
    setModerationModal({
      isOpen: true,
      actionType: "warning",
      error: undefined,
    })
  }

  const handleModeration = async (data: { reason: string; duration: string }) => {
    if (moderationModal.actionType === "mute") {
      const result = await actions.muteUser(user.id, {
        reason: data.reason,
        duration: data.duration,
      })

      if (result.ok) {
        setModerationModal(prev => ({ ...prev, isOpen: false }))
        await refreshStanding()
      } else if (result.error) {
        setModerationModal(prev => ({ ...prev, error: result.error }))
      }
    } else {
      const result = await actions.warnUser(user.id, {
        reason: data.reason,
        duration: data.duration,
      })

      if (result.ok) {
        setModerationModal(prev => ({ ...prev, isOpen: false }))
        await refreshStanding()
      } else if (result.error) {
        setModerationModal(prev => ({ ...prev, error: result.error }))
      }
    }
  }

  const handleBlockUser = async () => {
    const result = await actions.blockUser(user.id)
    if (result.ok) {
      refreshUser()
    }
  }

  const handleUnblockUser = async () => {
    const result = await actions.unblockUser(user.id)
    if (result.ok) {
      refreshUser()
    }
  }

  if (!canModerate && !canBlock) return null

  return (
    <>
      <div className="c-user-profile__actions">
        {canBlock && (
          <div className="c-user-profile__moderation">
            {!isBlocked ? (
              <button className="danger" onClick={handleBlockUser}>
                <Trans id="action.block">Block User</Trans>
              </button>
            ) : (
              <button className="secondary" onClick={handleUnblockUser}>
                <Trans id="action.unblock">Unblock User</Trans>
              </button>
            )}
          </div>
        )}
        {canModerate && !isBlocked && (
          <div className="c-user-profile__moderation">
            <button className="danger" onClick={openMuteModal}>
              <Trans id="action.mute">Mute User</Trans>
            </button>
            <button className="secondary" onClick={openWarnModal}>
              <Trans id="action.warn">Warn User</Trans>
            </button>
          </div>
        )}
      </div>

      <ModerationModal
        isOpen={moderationModal.isOpen}
        onClose={() => setModerationModal(prev => ({ ...prev, isOpen: false }))}
        actionType={moderationModal.actionType}
        onSubmit={handleModeration}
        error={moderationModal.error}
      />
    </>
  )
}

