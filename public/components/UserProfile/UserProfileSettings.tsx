import React, { useState, useEffect } from "react"
import { Modal, Form, Input, Button, Icon, Toggle } from "@fider/components"
import { useUserProfile } from "./context"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { UserSettings } from "@fider/models"
import { actions, Failure, Fider, cache } from "@fider/services"
import { NotificationSettings } from "@fider/pages/MySettings/components/NotificationSettings"
import { APIKeyForm } from "@fider/pages/MySettings/components/APIKeyForm"
import { DangerZone } from "@fider/pages/MySettings/components/DangerZone"
import { heroiconsMail as IconMail, heroiconsBell as IconBell, heroiconsKey as IconKey, heroiconsExclamation as IconWarning, heroiconsAdjustments as IconAdjustments } from "@fider/icons.generated"

const VOTE_POSITION_KEY = "fider_vote_position"

export const getVotePosition = (): "left" | "right" => {
  const stored = cache.local.get(VOTE_POSITION_KEY)
  return stored === "right" ? "right" : "left"
}

export const setVotePosition = (position: "left" | "right"): void => {
  cache.local.set(VOTE_POSITION_KEY, position)
}

interface UserProfileSettingsProps {
  userSettings?: UserSettings
}

interface SettingsState {
  showModal: boolean
  changingEmail: boolean
  newEmail: string
  error?: Failure
  userSettings: UserSettings
  votePosition: "left" | "right"
}

const UserProfileSettingsComponent: React.FC<UserProfileSettingsProps> = ({ userSettings }) => {
  const { isViewingOwnProfile, isEmbedded } = useUserProfile()

  const [settingsState, setSettingsState] = useState<SettingsState>({
    showModal: false,
    changingEmail: false,
    newEmail: "",
    userSettings: userSettings || {},
    votePosition: getVotePosition(),
  })

  const handleVotePositionChange = (isRight: boolean) => {
    const newPosition = isRight ? "right" : "left"
    setVotePosition(newPosition)
    setSettingsState(prev => ({ ...prev, votePosition: newPosition }))
  }

  if (!isViewingOwnProfile || isEmbedded) return null

  const confirmSettings = async () => {
    const settingsResult = await actions.updateUserSettings({
      settings: settingsState.userSettings,
    })
    if (settingsResult.ok) {
      location.reload()
    } else if (settingsResult.error) {
      setSettingsState(prev => ({ ...prev, error: settingsResult.error }))
    }
  }

  const submitNewEmail = async () => {
    const result = await actions.changeUserEmail(settingsState.newEmail)
    if (result.ok) {
      setSettingsState(prev => ({
        ...prev,
        error: undefined,
        changingEmail: false,
        showModal: true,
      }))
    } else if (result.error) {
      setSettingsState(prev => ({ ...prev, error: result.error }))
    }
  }

  const startChangeEmail = () => {
    setSettingsState(prev => ({ ...prev, changingEmail: true }))
  }

  const cancelChangeEmail = () => {
    setSettingsState(prev => ({
      ...prev,
      changingEmail: false,
      newEmail: "",
      error: undefined,
    }))
  }

  const setNewEmail = (newEmail: string) => {
    setSettingsState(prev => ({ ...prev, newEmail }))
  }

  const setNotificationSettings = (userSettings: UserSettings) => {
    setSettingsState(prev => ({ ...prev, userSettings }))
  }

  return (
    <div className="flex flex-col gap-6">
      <Modal.Window isOpen={settingsState.showModal} onClose={() => setSettingsState(prev => ({ ...prev, showModal: false }))}>
        <Modal.Header>
          <Trans id="modal.changeemail.header">Confirm your new email</Trans>
        </Modal.Header>
        <Modal.Content>
          <div>
            <p>
              <Trans id="modal.changeemail.text">
                We have just sent a confirmation link to <b>{settingsState.newEmail}</b>. <br /> Click the link to update your email.
              </Trans>
            </p>
            <p>
              <a href="#" onClick={() => setSettingsState(prev => ({ ...prev, showModal: false }))}>
                <Trans id="action.ok">OK</Trans>
              </a>
            </p>
          </div>
        </Modal.Content>
      </Modal.Window>

      <div className="bg-elevated rounded-card shadow-sm overflow-hidden">
        <div className="flex items-center gap-2 p-4 border-b border-surface-alt">
          <Icon sprite={IconMail} className="h-5 w-5 text-primary" />
          <h3 className="m-0 font-semibold">
            <Trans id="mysettings.email.title">Email Address</Trans>
          </h3>
        </div>
        <div className="p-4">
          <Form error={settingsState.error}>
            <Input
              label={i18n._("label.email", { message: "Email" })}
              field="email"
              value={settingsState.changingEmail ? settingsState.newEmail : Fider.session.user.email}
              maxLength={200}
              disabled={!settingsState.changingEmail}
              afterLabel={settingsState.changingEmail ? undefined : (
                <Button variant="tertiary" size="small" onClick={startChangeEmail}>
                  <Trans id="action.change">change</Trans>
                </Button>
              )}
              onChange={setNewEmail}
            >
              <p className="text-muted text-sm mt-1">
                {Fider.session.user.email || settingsState.changingEmail ? (
                  <Trans id="mysettings.message.privateemail">Your email is private and will never be publicly displayed.</Trans>
                ) : (
                  <Trans id="mysettings.message.noemail">Your account doesn&apos;t have an email.</Trans>
                )}
              </p>
              {settingsState.changingEmail && (
                <div className="flex gap-2 mt-3">
                  <Button variant="primary" size="small" onClick={submitNewEmail}>
                    <Trans id="action.confirm">Confirm</Trans>
                  </Button>
                  <Button variant="tertiary" size="small" onClick={cancelChangeEmail}>
                    <Trans id="action.cancel">Cancel</Trans>
                  </Button>
                </div>
              )}
            </Input>
          </Form>
        </div>
      </div>

      <div className="bg-elevated rounded-card shadow-sm overflow-hidden">
        <div className="flex items-center gap-2 p-4 border-b border-surface-alt">
          <Icon sprite={IconBell} className="h-5 w-5 text-primary" />
          <h3 className="m-0 font-semibold">
            <Trans id="label.notifications">Notifications</Trans>
          </h3>
        </div>
        <div className="p-4">
          <NotificationSettings 
            userSettings={userSettings || {}} 
            settingsChanged={setNotificationSettings} 
          />
          <div className="mt-4 pt-4 border-t border-surface-alt">
            <Button variant="primary" onClick={confirmSettings}>
              <Trans id="action.save">Save</Trans>
            </Button>
          </div>
        </div>
      </div>

      <div className="bg-elevated rounded-card shadow-sm overflow-hidden">
        <div className="flex items-center gap-2 p-4 border-b border-surface-alt">
          <Icon sprite={IconAdjustments} className="h-5 w-5 text-primary" />
          <h3 className="m-0 font-semibold">
            <Trans id="mysettings.display.title">Display</Trans>
          </h3>
        </div>
        <div className="p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium m-0">
                <Trans id="mysettings.display.voteposition">Vote Position</Trans>
              </p>
              <p className="text-muted text-sm m-0">
                <Trans id="mysettings.display.voteposition.description">Show votes on the right side of posts</Trans>
              </p>
            </div>
            <Toggle
              field="votePosition"
              active={settingsState.votePosition === "right"}
              onToggle={handleVotePositionChange}
            />
          </div>
        </div>
      </div>

      {Fider.session.user.isCollaborator && (
        <div className="bg-elevated rounded-card shadow-sm overflow-hidden">
          <div className="flex items-center gap-2 p-4 border-b border-surface-alt">
            <Icon sprite={IconKey} className="h-5 w-5 text-primary" />
            <h3 className="m-0 font-semibold">
              <Trans id="mysettings.apikey.title">API Key</Trans>
            </h3>
          </div>
          <div className="p-4">
            <APIKeyForm />
          </div>
        </div>
      )}

      <div className="bg-elevated rounded-card shadow-sm overflow-hidden border border-danger-light">
        <div className="flex items-center gap-2 p-4 border-b border-danger-light bg-danger-light">
          <Icon sprite={IconWarning} className="h-5 w-5 text-danger" />
          <h3 className="m-0 font-semibold text-danger">
            <Trans id="mysettings.dangerzone.title">Danger Zone</Trans>
          </h3>
        </div>
        <div className="p-4">
          <DangerZone />
        </div>
      </div>
    </div>
  )
}

UserProfileSettingsComponent.displayName = "UserProfileSettings"

export const UserProfileSettings = UserProfileSettingsComponent

