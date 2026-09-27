import React, { useState } from "react"
import { fireEvent, render, screen } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { NotificationSettings } from "./NotificationSettings"
import { useFider } from "@fider/hooks"
import { i18n } from "@lingui/core"
import { UserSettings } from "@fider/models"

jest.mock("@fider/hooks", () => ({ useFider: jest.fn() }))
jest.mock("@fider/services", () => ({
  push: { isPushSupported: () => false },
  classSet: () => "",
}))
jest.mock("@fider/components", () => ({
  Field: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Toggle: ({ label, active, onToggle }: { label: string; active: boolean; onToggle: () => void }) => (
    <button aria-pressed={active} onClick={onToggle}>{label}</button>
  ),
}))

beforeEach(() => {
  i18n.load("en", { "mysettings.notification.channelemail": "Email" })
  i18n.activate("en")
  jest.mocked(useFider).mockReturnValue({
    session: { permissions: { enableEmailNotifications: false } },
    settings: { notificationSubscriptions: {} },
  } as ReturnType<typeof useFider>)
})

test("an existing email preference can be disabled and restored before saving", () => {
  const saved = { event_notification_new_post: "3" }
  const Settings = () => {
    const [settings, setSettings] = useState<UserSettings>(saved)
    return (
      <>
        <NotificationSettings userSettings={saved} settingsChanged={setSettings} />
        <output>{settings.event_notification_new_post}</output>
      </>
    )
  }
  render(<Settings />)
  const email = screen.getByRole("button", { name: "Email" })
  expect(email).toHaveAttribute("aria-pressed", "true")

  fireEvent.click(email)
  expect(email).toHaveAttribute("aria-pressed", "false")
  expect(email).toBeVisible()
  expect(screen.getByRole("status")).toHaveTextContent("1")

  fireEvent.click(email)
  expect(email).toHaveAttribute("aria-pressed", "true")
  expect(screen.getByRole("status")).toHaveTextContent("3")
})

test("a saved opt-out cannot enable email without the server grant", () => {
  render(<NotificationSettings userSettings={{ event_notification_new_post: "1" }} settingsChanged={() => {}} />)
  expect(screen.queryByRole("button", { name: "Email" })).toBeNull()
})
