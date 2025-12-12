// UserProfileStanding converted to Tailwind

import React from "react"
import { Icon, Button } from "@fider/components"
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
      <div className="flex flex-col gap-6">
        <div className="flex flex-col items-center justify-center py-12 px-4 text-center text-success rounded-card min-h-[300px]">
          <Icon sprite={IconThumbsUp} className="h-16 w-16 mb-6" />
          <h4 className="text-2xl font-bold mb-3 tracking-wide">
            {isViewingOwnProfile ? (
              <Trans id="profile.standing.good.self">YOU ARE IN GOOD STANDING</Trans>
            ) : (
              <Trans id="profile.standing.good.other">
                {user.name} IS IN GOOD STANDING
              </Trans>
            )}
          </h4>
          <p className="text-lg text-muted">
            <Trans id="profile.standing.good.description">No warnings or mutes.</Trans>
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="bg-elevated rounded-card shadow-sm overflow-hidden">
        <h3 className="m-0 p-4 font-semibold flex items-center gap-2">
          <Icon sprite={IconWarning} className="h-5 w-5 text-warning" />
          <Trans id="profile.standing.warnings">Warnings</Trans>
        </h3>
        {standing.warnings.length === 0 ? (
          <p className="p-4 text-muted"><Trans id="profile.standing.nowarnings">No warnings</Trans></p>
        ) : (
          <div className="divide-y divide-surface-alt">
            {standing.warnings.map(warning => {
              const isExpired = warning.expiresAt && new Date(warning.expiresAt) < new Date()
              const isActive = warning.expiresAt && new Date(warning.expiresAt) > new Date()
              return (
                <div key={warning.id} className="flex items-start justify-between gap-4 p-4 transition-all hover:bg-tertiary max-md:flex-col">
                  <div className="flex-1 min-w-0">
                    <p className="m-0 mb-2 text-foreground text-[0.95em] leading-relaxed">{warning.reason}</p>
                    <div className="flex flex-wrap gap-3 mt-2">
                      <span className="flex items-center gap-1 text-muted text-[0.85em]">
                        <Icon sprite={IconCalendar} className="h-4 w-4 text-subtle" />
                        {new Date(warning.createdAt).toLocaleDateString()}
                      </span>
                      {warning.expiresAt && (
                        <span className="flex items-center gap-1 text-muted text-[0.85em]">
                          <Icon sprite={IconWarning} className="h-4 w-4 text-subtle" />
                          <Trans id="profile.standing.expires">
                            Expires: {new Date(warning.expiresAt).toLocaleDateString()}
                          </Trans>
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-col items-end gap-2 shrink-0 max-md:flex-row max-md:items-center max-md:w-full max-md:mt-2">
                    {(isExpired || isActive) && (
                      <span className={`px-2 py-1 rounded text-xs font-medium ${isExpired ? "bg-surface-alt text-muted" : "bg-warning-light text-warning"}`}>
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
          </div>
        )}
      </div>

      <div className="bg-elevated rounded-card shadow-sm overflow-hidden">
        <h3 className="m-0 p-4 font-semibold flex items-center gap-2">
          <Icon sprite={IconVolumeOff} className="h-5 w-5 text-warning" />
          <Trans id="profile.standing.mutes">Mutes</Trans>
        </h3>
        {standing.mutes.length === 0 ? (
          <p className="p-4 text-muted"><Trans id="profile.standing.nomutes">No mutes</Trans></p>
        ) : (
          <div className="divide-y divide-surface-alt">
            {standing.mutes.map(mute => {
              const isExpired = mute.expiresAt && new Date(mute.expiresAt) < new Date()
              const isActive = mute.expiresAt && new Date(mute.expiresAt) > new Date()
              return (
                <div key={mute.id} className="flex items-start justify-between gap-4 p-4 transition-all hover:bg-tertiary max-md:flex-col">
                  <div className="flex-1 min-w-0">
                    <p className="m-0 mb-2 text-foreground text-[0.95em] leading-relaxed">{mute.reason}</p>
                    <div className="flex flex-wrap gap-3 mt-2">
                      <span className="flex items-center gap-1 text-muted text-[0.85em]">
                        <Icon sprite={IconCalendar} className="h-4 w-4 text-subtle" />
                        {new Date(mute.createdAt).toLocaleDateString()}
                      </span>
                      {mute.expiresAt && (
                        <span className="flex items-center gap-1 text-muted text-[0.85em]">
                          <Icon sprite={IconVolumeOff} className="h-4 w-4 text-subtle" />
                          <Trans id="profile.standing.expires">
                            Expires: {new Date(mute.expiresAt).toLocaleDateString()}
                          </Trans>
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-col items-end gap-2 shrink-0 max-md:flex-row max-md:items-center max-md:w-full max-md:mt-2">
                    {(isExpired || isActive) && (
                      <span className={`px-2 py-1 rounded text-xs font-medium ${isExpired ? "bg-surface-alt text-muted" : "bg-warning-light text-warning"}`}>
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
          </div>
        )}
      </div>
    </div>
  )
}

UserProfileStandingComponent.displayName = "UserProfileStanding"

export const UserProfileStanding = UserProfileStandingComponent
