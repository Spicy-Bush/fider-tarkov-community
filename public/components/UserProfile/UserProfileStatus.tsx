// UserProfileStatus converted to Tailwind

import React from "react"
import { Icon } from "@fider/components"
import { useUserProfile } from "./context"
import { Trans } from "@lingui/react/macro"
import { UserStatus } from "@fider/models"
import { heroiconsBlock as IconBan, heroiconsMuted as IconVolumeOff } from "@fider/icons.generated"

export const UserProfileStatus: React.FC = () => {
  const { user, standing } = useUserProfile()

  if (!user) return null

  const isBlocked = user.status === UserStatus.Blocked || user.status === 2
  const activeMutes = standing.mutes.filter(
    mute => !mute.expiresAt || new Date(mute.expiresAt) > new Date()
  )

  if (!isBlocked && activeMutes.length === 0) return null

  return (
    <>
      {isBlocked && (
        <div className="col-span-full flex items-center gap-2 px-5 py-3 -mt-px rounded-b-lg shadow-sm bg-danger-light text-danger-hover max-md:p-3">
          <Icon sprite={IconBan} className="h-4" />
          <span>
            <Trans id="profile.status.blocked">This user is blocked</Trans>
          </span>
        </div>
      )}
      {activeMutes.map(mute => (
        <div key={mute.id} className="col-span-full flex items-center gap-2 px-5 py-3 -mt-px rounded-b-lg shadow-sm bg-warning-light text-warning max-md:p-3">
          <Icon sprite={IconVolumeOff} className="h-4" />
          <span>
            <Trans id="profile.status.muted">
              Muted until {new Date(mute.expiresAt!).toLocaleString()}
            </Trans>
            {mute.reason && (
              <span className="ml-2 text-sm opacity-80">
                <Trans id="profile.status.reason">Reason: {mute.reason}</Trans>
              </span>
            )}
          </span>
        </div>
      ))}
    </>
  )
}
