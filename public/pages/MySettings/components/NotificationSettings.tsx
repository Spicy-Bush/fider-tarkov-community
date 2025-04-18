import React, { useState } from "react"

import { UserSettings } from "@fider/models"
import { Toggle, Field } from "@fider/components"
import { useFider } from "@fider/hooks"
import { HStack, VStack } from "@fider/components/layout"
import { i18n } from "@lingui/core"
import { t, Trans } from "@lingui/macro"

type Channel = number
const WebChannel: Channel = 1
const EmailChannel: Channel = 2

interface NotificationSettingsProps {
  userSettings: UserSettings
  settingsChanged: (settings: UserSettings) => void
}

export const NotificationSettings = (props: NotificationSettingsProps) => {
  const fider = useFider()
  const [userSettings, setUserSettings] = useState(props.userSettings)

  const isEnabled = (settingsKey: string, channel: Channel): boolean => {
    if (settingsKey in userSettings) {
      return (parseInt(userSettings[settingsKey], 10) & channel) > 0
    }
    return false
  }

  const toggle = async (settingsKey: string, channel: Channel) => {
    const nextSettings = {
      ...userSettings,
      [settingsKey]: (parseInt(userSettings[settingsKey], 10) ^ channel).toString(),
    }
    setUserSettings(nextSettings)
    props.settingsChanged(nextSettings)
  }

  const labelWeb = i18n._("mysettings.notification.channelweb", { message: "Web" })
  const labelEmail = i18n._("mysettings.notification.channelemail", { message: "Email" })

  const icon = (settingsKey: string, channel: Channel) => {
    const active = isEnabled(settingsKey, channel)
    const label = channel === WebChannel ? labelWeb : labelEmail
    const onToggle = () => toggle(settingsKey, channel)
    return <Toggle key={`${settingsKey}_${channel}`} active={active} label={label} onToggle={onToggle} />
  }

  const info = (settingsKey: string, aboutForVisitors: string, aboutForCollaborators: string) => {
    const about = fider.session.user.isCollaborator ? aboutForCollaborators : aboutForVisitors
    const webEnabled = isEnabled(settingsKey, WebChannel)
    const emailEnabled = isEnabled(settingsKey, EmailChannel)

    if (!webEnabled && !emailEnabled) {
      return (
        <p className="text-muted">
          <Trans id="mysettings.notification.message.none">
            You&apos;ll <strong>NOT</strong> receive any notification about this event.
          </Trans>
        </p>
      )
    } else if (webEnabled && !emailEnabled) {
      return (
        <p className="text-muted">
          <Trans id="mysettings.notification.message.webonly">
            You&apos;ll receive <strong>web</strong> notifications about {about}.
          </Trans>
        </p>
      )
    } else if (!webEnabled && emailEnabled) {
      return (
        <p className="text-muted">
          <Trans id="mysettings.notification.message.emailonly">
            You&apos;ll receive <strong>email</strong> notifications about {about}.
          </Trans>
        </p>
      )
    } else if (webEnabled && emailEnabled) {
      return (
        <p className="text-muted">
          <Trans id="mysettings.notification.message.webandemail">
            You&apos;ll receive <strong>web</strong> and <strong>email</strong> notifications about {about}.
          </Trans>
        </p>
      )
    }
    return null
  }

  return (
    <>
      <Field label={i18n._("label.notifications", { message: "Notifications" })}>
        <p className="text-muted mb-6">
          <Trans id="mysettings.notification.title">
            Choose the events to receive a notification for.
          </Trans>
        </p>

        <div className="notifications-settings mt-4">
          <VStack spacing={4} divide={true} className="rounded">
            <div>
              <div className="mb-1">
                <Trans id="mysettings.notification.event.newpost">New Post</Trans>
              </div>
              {info(
                "event_notification_new_post",
                t({ id: "mysettings.notification.event.newpost.visitors", message: "new posts on this site" }),
                t({ id: "mysettings.notification.event.newpost.staff", message: "new posts on this site" })
              )}
              <HStack spacing={6}>
                {icon("event_notification_new_post", WebChannel)}
                {(fider.session.user.isCollaborator || fider.session.user.isAdministrator) && icon("event_notification_new_post", EmailChannel)}
              </HStack>
            </div>
            <div>
              <div className="mb-1">
                <Trans id="mysettings.notification.event.discussion">Discussion</Trans>
              </div>
              {info(
                "event_notification_new_comment",
                t({ id: "mysettings.notification.event.discussion.visitors", message: "comments on posts you've subscribed to" }),
                t({ id: "mysettings.notification.event.discussion.staff", message: "comments on all posts unless individually unsubscribed" })
              )}
              <HStack spacing={6}>
                {icon("event_notification_new_comment", WebChannel)}
                {(fider.session.user.isCollaborator || fider.session.user.isAdministrator) && icon("event_notification_new_comment", EmailChannel)}
              </HStack>
            </div>
            <div>
              <div className="mb-1">
                <Trans id="mysettings.notification.event.statuschanged">Status Changed</Trans>
              </div>
              {info(
                "event_notification_change_status",
                t({ id: "mysettings.notification.event.statuschanged.visitors", message: "status change on posts you've subscribed to" }),
                t({ id: "mysettings.notification.event.statuschanged.staff", message: "status change on all posts unless individually unsubscribed" })
              )}
              <HStack spacing={6}>
                {icon("event_notification_change_status", WebChannel)}
                {(fider.session.user.isCollaborator || fider.session.user.isAdministrator) && icon("event_notification_change_status", EmailChannel)}
              </HStack>
            </div>
          </VStack>
        </div>
      </Field>
    </>
  )
}
