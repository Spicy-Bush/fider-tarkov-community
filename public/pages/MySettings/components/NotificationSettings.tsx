import React, { useState, useEffect, useCallback } from "react"

import { UserSettings } from "@fider/models"
import { Toggle, Field, Button } from "@fider/components"
import { useFider } from "@fider/hooks"
import { HStack } from "@fider/components/layout"
import { i18n } from "@lingui/core"
import { t, Trans } from "@lingui/macro"
import { push } from "@fider/services"

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
  const [pushEnabled, setPushEnabled] = useState(false)
  const [pushSubscribed, setPushSubscribed] = useState(false)
  const [pushLoading, setPushLoading] = useState(false)
  const [pushPermission, setPushPermission] = useState<NotificationPermission>("default")

  useEffect(() => {
    const checkPush = async () => {
      if (!push.isPushSupported()) return

      setPushPermission(push.getNotificationPermission())
      
      const vapidKey = await push.getVAPIDPublicKey()
      setPushEnabled(vapidKey.enabled)
      
      if (vapidKey.enabled) {
        const subscription = await push.getCurrentSubscription()
        setPushSubscribed(!!subscription)
      }
    }
    checkPush()
  }, [])

  const handlePushToggle = useCallback(async () => {
    setPushLoading(true)
    try {
      if (pushSubscribed) {
        const result = await push.unsubscribeFromPush()
        if (result.ok) {
          setPushSubscribed(false)
        }
      } else {
        const result = await push.subscribeToPush()
        if (result.ok) {
          setPushSubscribed(true)
          setPushPermission("granted")
        }
      }
    } finally {
      setPushLoading(false)
    }
  }, [pushSubscribed])

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

    if (!webEnabled) {
      return (
        <p className="text-muted text-sm mt-1 mb-2">
          <Trans id="mysettings.notification.message.none">
            You&apos;ll <strong>NOT</strong> receive any notification about this event.
          </Trans>
        </p>
      )
    }
    return (
      <p className="text-muted text-sm mt-1 mb-2">
        <Trans id="mysettings.notification.message.enabled">
          You&apos;ll receive notifications about {about}.
        </Trans>
      </p>
    )
  }

  const renderPushSection = () => {
    if (!push.isPushSupported()) {
      return null
    }

    if (!pushEnabled) {
      return null
    }

    return (
      <div className="p-4 bg-elevated">
        <div className="font-medium mb-1">
          <Trans id="mysettings.notification.push.title">Push Notifications</Trans>
        </div>
        <p className="text-muted text-sm mt-1 mb-2">
          {pushSubscribed ? (
            <Trans id="mysettings.notification.push.enabled">
              Push notifications are enabled for this device.
            </Trans>
          ) : pushPermission === "denied" ? (
            <Trans id="mysettings.notification.push.blocked">
              Push notifications are blocked. Enable them in your browser settings.
            </Trans>
          ) : (
            <Trans id="mysettings.notification.push.disabled">
              Enable push notifications to receive alerts even when the site is closed.
            </Trans>
          )}
        </p>
        <Button
          variant={pushSubscribed ? "secondary" : "primary"}
          size="small"
          disabled={pushLoading || pushPermission === "denied"}
          onClick={handlePushToggle}
        >
          {pushLoading ? (
            <Trans id="mysettings.notification.push.loading">Loading...</Trans>
          ) : pushSubscribed ? (
            <Trans id="mysettings.notification.push.disable">Disable Push</Trans>
          ) : (
            <Trans id="mysettings.notification.push.enable">Enable Push</Trans>
          )}
        </Button>
      </div>
    )
  }

  return (
    <Field label={i18n._("label.notifications", { message: "Notifications" })}>
      <p className="text-muted text-sm mb-4">
        <Trans id="mysettings.notification.title">
          Choose the events to receive a notification for.
        </Trans>
      </p>

      <div className="divide-y divide-surface-alt border border-surface-alt rounded-card overflow-hidden">
        {renderPushSection()}
        <div className="p-4 bg-elevated">
          <div className="font-medium mb-1">
            <Trans id="mysettings.notification.event.newpost">New Post</Trans>
          </div>
          {info(
            "event_notification_new_post",
            t({ id: "mysettings.notification.event.newpost.visitors", message: "new posts on this site" }),
            t({ id: "mysettings.notification.event.newpost.staff", message: "new posts on this site" })
          )}
          <HStack spacing={6}>
            {icon("event_notification_new_post", WebChannel)}
            {fider.session.user.isAdministrator && icon("event_notification_new_post", EmailChannel)}
          </HStack>
        </div>
        <div className="p-4 bg-elevated">
          <div className="font-medium mb-1">
            <Trans id="mysettings.notification.event.discussion">Discussion</Trans>
          </div>
          {info(
            "event_notification_new_comment",
            t({ id: "mysettings.notification.event.discussion.visitors", message: "comments on posts you've subscribed to" }),
            t({ id: "mysettings.notification.event.discussion.staff", message: "comments on all posts unless individually unsubscribed" })
          )}
          <HStack spacing={6}>
            {icon("event_notification_new_comment", WebChannel)}
            {fider.session.user.isAdministrator && icon("event_notification_new_comment", EmailChannel)}
          </HStack>
        </div>
        <div className="p-4 bg-elevated">
          <div className="font-medium mb-1">
            <Trans id="mysettings.notification.event.mention">Mentions</Trans>
          </div>
          {info(
            "event_notification_mention",
            t({ id: "mysettings.notification.event.mention.description", message: "when someone mentions you in a comment" }),
            t({ id: "mysettings.notification.event.mention.description", message: "when someone mentions you in a comment" })
          )}
          <HStack spacing={6}>
            {icon("event_notification_mention", WebChannel)}
            {fider.session.user.isAdministrator && icon("event_notification_mention", EmailChannel)}
          </HStack>
        </div>
        <div className="p-4 bg-elevated">
          <div className="font-medium mb-1">
            <Trans id="mysettings.notification.event.statuschanged">Status Changed</Trans>
          </div>
          {info(
            "event_notification_change_status",
            t({ id: "mysettings.notification.event.statuschanged.visitors", message: "status change on posts you've subscribed to" }),
            t({ id: "mysettings.notification.event.statuschanged.staff", message: "status change on all posts unless individually unsubscribed" })
          )}
          <HStack spacing={6}>
            {icon("event_notification_change_status", WebChannel)}
            {fider.session.user.isAdministrator && icon("event_notification_change_status", EmailChannel)}
          </HStack>
        </div>
      </div>
    </Field>
  )
}
