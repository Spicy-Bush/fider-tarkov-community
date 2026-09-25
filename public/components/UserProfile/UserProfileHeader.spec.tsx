import React from "react"
import { i18n } from "@lingui/core"
import { I18nProvider } from "@lingui/react"
import { afterEach, expect, jest, test } from "@jest/globals"
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { Fider, actions, http } from "@fider/services"
import { UserAvatarType, UserRole, UserStatus } from "@fider/models"
import { UserMenu } from "../auth/UserMenu"
import { UserProfileProvider } from "./context"
import { UserProfileHeader } from "./UserProfileHeader"

jest.mock("@fider/services/actions")
jest.unmock("@lingui/react")

afterEach(() => {
  jest.restoreAllMocks()
  jest.clearAllMocks()
  document.getElementById("root-modal")?.remove()
})

test.each([1, 2])("saving user %i’s avatar updates their profile without changing another user’s menu", async (userId) => {
  i18n.load("en", {})
  i18n.activate("en")

  const modalRoot = document.createElement("div")
  modalRoot.id = "root-modal"
  document.body.appendChild(modalRoot)

  const user = {
    id: 1,
    name: "Local Admin",
    role: UserRole.Administrator,
    status: UserStatus.Active,
    avatarType: UserAvatarType.Custom,
    avatarURL: "/static/images/avatars/old.png",
    isAdministrator: true,
  }
  Fider.initialize({ user, tenant: {}, settings: {} })
  let published = { name: user.name, avatarType: user.avatarType, avatarURL: user.avatarURL }

  jest.mocked(actions.getUserProfileStats).mockResolvedValue({ ok: true, data: { posts: 0, comments: 0, votes: 0 } })
  jest.mocked(actions.getUserProfileStanding).mockResolvedValue({ ok: true, data: { warnings: [], mutes: [] } })
  jest.spyOn(http, "get").mockImplementation(async () => ({ ok: true, data: { ...published, changes: [] } }))
  const save = jest.mocked(actions.updateUserAvatar).mockImplementation(async ({ avatarType }) => {
    published = { name: user.name, avatarType, avatarURL: avatarType === UserAvatarType.Letter ? "" : "/static/avatars/gravatar/1" }
    return { ok: true, data: { ...published, pending: false } }
  })

  const view = render(
    <I18nProvider i18n={i18n}>
      <UserMenu />
      <UserProfileProvider userId={userId} user={{ ...user, id: userId }} embedded>
        <UserProfileHeader />
      </UserProfileProvider>
    </I18nProvider>
  )

  await waitFor(() => expect(actions.getUserProfileStanding).toHaveBeenCalled())
  const menu = view.container.querySelector(".c-menu-user")!
  const profileAvatar = view.container.querySelector('div[style="cursor: pointer;"]')!
  expect(menu.querySelector("img")).not.toBeNull()
  expect(profileAvatar.querySelector("img")).not.toBeNull()

  fireEvent.click(profileAvatar)
  fireEvent.click(within(screen.getByRole("dialog")).getByText("Letter"))
  fireEvent.click(within(screen.getByRole("dialog")).getByText("Save"))

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
  if (userId === 1) {
    expect(menu.querySelector("img")).toBeNull()
    expect(menu.querySelector("text")?.textContent).toBe("LA")
  } else {
    expect(menu.querySelector("img")?.getAttribute("src")).toBe("/static/images/avatars/old.png?size=64")
  }

  expect(profileAvatar.querySelector("img")).toBeNull()
  expect(profileAvatar.querySelector("text")?.textContent).toBe("LA")
  expect(save).toHaveBeenCalledTimes(1)
  expect(screen.queryByText(/awaiting a check/)).toBeNull()

  fireEvent.click(profileAvatar)
  fireEvent.click(within(screen.getByRole("dialog")).getByText("Gravatar"))
  fireEvent.click(within(screen.getByRole("dialog")).getByText("Save"))

  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
  const menuURL = userId === 1 ? "/static/avatars/gravatar/1?size=64" : "/static/images/avatars/old.png?size=64"
  expect(menu.querySelector("img")?.getAttribute("src")).toBe(menuURL)
  expect(profileAvatar.querySelector("img")?.getAttribute("src")).toBe("/static/avatars/gravatar/1?size=200")
  expect(save).toHaveBeenCalledTimes(2)
  expect(Fider.session.user.avatarType).toBe(userId === 1 ? UserAvatarType.Gravatar : UserAvatarType.Custom)
})