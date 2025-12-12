import React from "react"
import { Icon, Button } from "@fider/components"
import { VStack } from "@fider/components/layout"
import { useUserProfile } from "./context"
import { Trans } from "@lingui/react/macro"
import { actions, notify } from "@fider/services"
import { i18n } from "@lingui/core"
import { heroiconsCalendar as IconCalendar, heroiconsExclamation as IconWarning, heroiconsMuted as IconVolumeOff, heroiconsThumbsup as IconThumbsUp } from "@fider/icons.generated"

const UserProfileStandingComponent: React.FC = () => {
  const { user, standing, canDeleteModeration, canModerate, refreshStanding, isViewingOwnProfile } = useUserProfile()

  if (!user) return null

  const handleDeleteWarning = async (warningId: number) => {
    const result = await actions.deleteWarning(user.id, warningId)
    if (result.ok) {
      await refreshStanding()
      notify.success(i18n._("profile.warning.delete.success", { message: "Warning has been deleted successfully" }))
    }
  }

  const handleDeleteMute = async (muteId: number) => {
    const result = await actions.deleteMute(user.id, muteId)
    if (result.ok) {
      await refreshStanding()
      notify.success(i18n._("profile.mute.delete.success", { message: "Mute has been deleted successfully" }))
    }
  }

  const handleExpireWarning = async (warningId: number) => {
    const result = await actions.expireWarning(user.id, warningId)
    if (result.ok) {
      await refreshStanding()
      notify.success(i18n._("profile.warning.expire.success", { message: "Warning has been removed" }))
    }
  }

  const handleExpireMute = async (muteId: number) => {
    const result = await actions.expireMute(user.id, muteId)
    if (result.ok) {
      await refreshStanding()
      notify.success(i18n._("profile.mute.expire.success", { message: "User has been unmuted" }))
    }
  }

  if (standing.warnings.length === 0 && standing.mutes.length === 0) {
    return (
      <div className="c-user-profile__standing">
        <div className="c-user-profile__good-standing">
          <Icon sprite={IconThumbsUp} className="h-8" />
          <h4 className="c-user-profile__good-standing-title">
            {isViewingOwnProfile ? (
              <Trans id="profile.standing.good.self">YOU ARE IN GOOD STANDING</Trans>
            ) : (
              <Trans id="profile.standing.good.other">
                {user.name} IS IN GOOD STANDING
              </Trans>
            )}
          </h4>
          <p className="c-user-profile__good-standing-subtitle">
            <Trans id="profile.standing.good.description">No warnings or mutes.</Trans>
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="c-user-profile__standing">
      <div className="c-user-profile__standing-section">
        <h3>
          <Icon sprite={IconWarning} className="h-4" />
          <Trans id="profile.standing.warnings">Warnings</Trans>
        </h3>
        {standing.warnings.length === 0 ? (
          <p className="text-muted"><Trans id="profile.standing.nowarnings">No warnings</Trans></p>
        ) : (
          <VStack spacing={4} divide>
            {standing.warnings.map(warning => {
              const isExpired = warning.expiresAt && new Date(warning.expiresAt) < new Date()
              const isActive = warning.expiresAt && new Date(warning.expiresAt) > new Date()
              return (
                <div key={warning.id} className="c-user-profile__standing-item">
                  <div className="c-user-profile__standing-content">
                    <p>{warning.reason}</p>
                    <div className="c-user-profile__standing-meta">
                      <span className="meta-item">
                        <Icon sprite={IconCalendar} className="h-4" />
                        {new Date(warning.createdAt).toLocaleDateString()}
                      </span>
                      {warning.expiresAt && (
                        <span className="meta-item">
                          <Icon sprite={IconWarning} className="h-4" />
                          <Trans id="profile.standing.expires">
                            Expires: {new Date(warning.expiresAt).toLocaleDateString()}
                          </Trans>
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="c-user-profile__standing-actions">
                    {(isExpired || isActive) && (
                      <span className={`c-user-profile__standing-status ${isExpired ? "expired" : "active"}`}>
                        {isExpired ? "Expired" : "Active"}
                      </span>
                    )}
                    {isActive && canModerate && (
                      <Button variant="secondary" size="small" onClick={() => handleExpireWarning(warning.id)}>
                        <Trans id="action.removeWarning">Remove</Trans>
                      </Button>
                    )}
                    {canDeleteModeration && (
                      <Button variant="danger" size="small" onClick={() => handleDeleteWarning(warning.id)}>
                        <Trans id="action.delete">Delete</Trans>
                      </Button>
                    )}
                  </div>
                </div>
              )
            })}
          </VStack>
        )}
      </div>

      <div className="c-user-profile__standing-section">
        <h3>
          <Icon sprite={IconVolumeOff} className="h-4" />
          <Trans id="profile.standing.mutes">Mutes</Trans>
        </h3>
        {standing.mutes.length === 0 ? (
          <p className="text-muted"><Trans id="profile.standing.nomutes">No mutes</Trans></p>
        ) : (
          <VStack spacing={4} divide>
            {standing.mutes.map(mute => {
              const isExpired = mute.expiresAt && new Date(mute.expiresAt) < new Date()
              const isActive = mute.expiresAt && new Date(mute.expiresAt) > new Date()
              return (
                <div key={mute.id} className="c-user-profile__standing-item">
                  <div className="c-user-profile__standing-content">
                    <p>{mute.reason}</p>
                    <div className="c-user-profile__standing-meta">
                      <span className="meta-item">
                        <Icon sprite={IconCalendar} className="h-4" />
                        {new Date(mute.createdAt).toLocaleDateString()}
                      </span>
                      {mute.expiresAt && (
                        <span className="meta-item">
                          <Icon sprite={IconVolumeOff} className="h-4" />
                          <Trans id="profile.standing.expires">
                            Expires: {new Date(mute.expiresAt).toLocaleDateString()}
                          </Trans>
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="c-user-profile__standing-actions">
                    {(isExpired || isActive) && (
                      <span className={`c-user-profile__standing-status ${isExpired ? "expired" : "active"}`}>
                        {isExpired ? "Expired" : "Active"}
                      </span>
                    )}
                    {isActive && canModerate && (
                      <Button variant="secondary" size="small" onClick={() => handleExpireMute(mute.id)}>
                        <Trans id="action.unmute">Unmute</Trans>
                      </Button>
                    )}
                    {canDeleteModeration && (
                      <Button variant="danger" size="small" onClick={() => handleDeleteMute(mute.id)}>
                        <Trans id="action.delete">Delete</Trans>
                      </Button>
                    )}
                  </div>
                </div>
              )
            })}
          </VStack>
        )}
      </div>
    </div>
  )
}

UserProfileStandingComponent.displayName = "UserProfileStanding"

export const UserProfileStanding = UserProfileStandingComponent

