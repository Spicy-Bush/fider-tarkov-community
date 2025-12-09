import React, { useState } from "react"
import { Modal, Form, Input, Button } from "@fider/components"
import { useUserProfile } from "./context"
import { Trans } from "@lingui/react/macro"
import { i18n } from "@lingui/core"
import { UserSettings } from "@fider/models"
import { actions, Failure, Fider } from "@fider/services"
import { NotificationSettings } from "@fider/pages/MySettings/components/NotificationSettings"
import { APIKeyForm } from "@fider/pages/MySettings/components/APIKeyForm"
import { DangerZone } from "@fider/pages/MySettings/components/DangerZone"

interface UserProfileSettingsProps {
  userSettings?: UserSettings
}

interface SettingsState {
  showModal: boolean
  changingEmail: boolean
  newEmail: string
  error?: Failure
  userSettings: UserSettings
}

const UserProfileSettingsComponent: React.FC<UserProfileSettingsProps> = ({ userSettings }) => {
  const { isViewingOwnProfile, isEmbedded } = useUserProfile()

  const [settingsState, setSettingsState] = useState<SettingsState>({
    showModal: false,
    changingEmail: false,
    newEmail: "",
    userSettings: userSettings || {},
  })

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
    <>
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
          <p className="text-muted">
            {Fider.session.user.email || settingsState.changingEmail ? (
              <Trans id="mysettings.message.privateemail">Your email is private and will never be publicly displayed.</Trans>
            ) : (
              <Trans id="mysettings.message.noemail">Your account doesn&apos;t have an email.</Trans>
            )}
          </p>
          {settingsState.changingEmail && (
            <>
              <Button variant="primary" size="small" onClick={submitNewEmail}>
                <Trans id="action.confirm">Confirm</Trans>
              </Button>
              <Button variant="tertiary" size="small" onClick={cancelChangeEmail}>
                <Trans id="action.cancel">Cancel</Trans>
              </Button>
            </>
          )}
        </Input>

        <NotificationSettings 
          userSettings={userSettings || {}} 
          settingsChanged={setNotificationSettings} 
        />

        <Button variant="primary" onClick={confirmSettings}>
          <Trans id="action.save">Save</Trans>
        </Button>
      </Form>

      <div className="mt-8">
        {Fider.session.user.isCollaborator && <APIKeyForm />}
      </div>
      <div className="mt-8">
        <DangerZone />
      </div>
    </>
  )
}

UserProfileSettingsComponent.displayName = "UserProfileSettings"

export const UserProfileSettings = UserProfileSettingsComponent

