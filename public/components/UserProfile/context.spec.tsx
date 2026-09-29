import React from "react"
import { noSessionPermissions, noUserPermissions } from "@fider/services/testing/permissions"
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react"
import { expect, jest, test } from "@jest/globals"
import { UserRole, UserStatus, VisualRole } from "@fider/models"
import { Fider } from "@fider/services/fider"
import * as actions from "@fider/services/actions/user"
import { changeUserVisualRole } from "@fider/services/actions/tenant"
import { RequestError } from "@fider/services/http"
import { UserStandingProvider } from "@fider/contexts/UserStandingContext"
import { UserProfileProvider, useUserProfile } from "./context"
import { UserProfile } from "./UserProfile"
import { UserProfileDetails } from "./UserProfileDetails"

jest.mock("@fider/services/actions/user")
jest.mock("@fider/services/actions/tenant")

test.each([7, 8])("profile %i retries both stats and standing after a failed initial read", async (userId) => {
  const user = { permissions: { ...noUserPermissions, editAvatar: true },
    id: 7,
    name: "Member",
    role: UserRole.Visitor,
    status: UserStatus.Active,
    avatarURL: "",
  }
  Fider.initialize({ permissions: noSessionPermissions, user, contextID: "profile", tenant: {}, settings: {} })

  jest.mocked(actions.getUserProfileStanding).mockReset().mockResolvedValue({ ok: true, data: { warnings: [], mutes: [], sessionPermissions: noSessionPermissions } })
  jest.mocked(actions.getUserProfileStats).mockReset()
    .mockRejectedValueOnce(new RequestError("GET", `/api/user/profile/${userId}/stats`, "transport", new Error("Disconnected")))
    .mockResolvedValue({ ok: true, data: { posts: 3, comments: 5, votes: 8 } })

  const { result } = renderHook(useUserProfile, {
    wrapper: ({ children }) => (
      <UserStandingProvider>
        <UserProfileProvider userId={userId} user={{ ...user, id: userId }} embedded>
          {children}
        </UserProfileProvider>
      </UserStandingProvider>
    ),
  })

  await waitFor(() => expect(result.current.error).toBe("Could not load profile stats."))
  await act(() => result.current.refreshProfile())

  expect(result.current.error).toBeNull()
  expect(result.current.stats).toEqual({ posts: 3, comments: 5, votes: 8 })
  expect(actions.getUserProfileStanding).toHaveBeenCalledTimes(2)
})

test("switching embedded profiles discards the previous identity and its pending stats", async () => {
  const user = { permissions: { ...noUserPermissions, editAvatar: true }, id: 7, name: "Member", role: UserRole.Visitor, status: UserStatus.Active, avatarURL: "" }
  Fider.initialize({ permissions: noSessionPermissions, user, contextID: "profile", tenant: {}, settings: {} })

  let completeOld: (result: Awaited<ReturnType<typeof actions.getUserProfileStats>>) => void
  jest.mocked(actions.getUserProfileStats).mockReset()
    .mockReturnValueOnce(new Promise((resolve) => {
      completeOld = resolve
    }))
    .mockResolvedValue({ ok: true, data: { posts: 9, comments: 0, votes: 0 } })
  jest.mocked(actions.getUserProfileStanding).mockReset().mockResolvedValue({ ok: true, data: { warnings: [], mutes: [], sessionPermissions: noSessionPermissions } })

  const ProfileIdentity = () => {
    const profile = useUserProfile()
    return <output>{profile.user?.id}:{profile.stats.posts}</output>
  }
  const show = (userId: number) => (
    <UserStandingProvider>
      <UserProfile userId={userId} user={{ ...user, id: userId }} embedded>
        <ProfileIdentity />
      </UserProfile>
    </UserStandingProvider>
  )

  const { rerender } = render(show(8))
  rerender(show(9))
  await waitFor(() => expect(screen.getByText("9:9")).toBeVisible())

  await act(async () => {
    completeOld!({ ok: true, data: { posts: 8, comments: 0, votes: 0 } })
  })

  expect(screen.getByText("9:9")).toBeVisible()
})

test("a confirmed role receipt replaces target permissions while the profile stays mounted", async () => {
  const actor = { id: 7, name: "Admin", role: UserRole.Administrator, permissions: noUserPermissions }
  const target = {
    id: 8,
    name: "Member",
    role: UserRole.Helper,
    status: UserStatus.Active,
    avatarURL: "",
    permissions: { ...noUserPermissions, moderate: true, block: true },
  }
  Fider.initialize({ permissions: noSessionPermissions, user: actor, contextID: "profile", tenant: {}, settings: {} })
  jest.mocked(actions.getUserProfileStanding).mockReset().mockResolvedValue({
    ok: true,
    data: { warnings: [], mutes: [], sessionPermissions: noSessionPermissions },
  })
  jest.mocked(actions.getUserProfileStats).mockReset().mockResolvedValue({ ok: true, data: { posts: 0, comments: 0, votes: 0 } })

  const Access = () => {
    const profile = useUserProfile()
    return <output>{profile.activeTab}:{String(profile.canModerate)}:{String(profile.canBlock)}</output>
  }
  const show = (user: typeof target) => (
    <UserStandingProvider>
      <UserProfileProvider userId={user.id} user={user} embedded>
        <Access />
      </UserProfileProvider>
    </UserStandingProvider>
  )
  const { rerender } = render(show(target))
  expect(screen.getByText("search:true:true")).toBeVisible()

  rerender(show({ ...target, role: UserRole.Administrator, permissions: noUserPermissions }))
  await waitFor(() => expect(screen.getByText("search:false:false")).toBeVisible())
  expect(actions.getUserProfileStats).toHaveBeenCalledTimes(1)
})

test.each([7, 8])("badge changes for user %i recover after rejection and update the correct profile owner", async (userId) => {
  const actor = {
    id: 7,
    name: "Admin",
    role: UserRole.Administrator,
    visualRole: VisualRole.Administrator,
    visualRoleOverride: VisualRole.None,
    status: UserStatus.Active,
    avatarURL: "",
    permissions: { ...noUserPermissions, changeVisualRole: true },
  }
  Fider.initialize({ permissions: noSessionPermissions, user: actor, contextID: "profile", tenant: {}, settings: {} })
  jest.mocked(actions.getUserProfileStanding).mockReset().mockResolvedValue({
    ok: true,
    data: { warnings: [], mutes: [], sessionPermissions: noSessionPermissions },
  })
  jest.mocked(actions.getUserProfileStats).mockReset().mockResolvedValue({ ok: true, data: { posts: 0, comments: 0, votes: 0 } })
  const save = jest.mocked(changeUserVisualRole).mockReset()
    .mockResolvedValueOnce({ ok: false, error: { errors: [{ message: "Badge change rejected." }] } })
    .mockResolvedValueOnce({ ok: true, data: { id: userId, visualRole: VisualRole.Sherpa, visualRoleOverride: VisualRole.Sherpa } })
    .mockResolvedValueOnce({ ok: true, data: { id: userId, visualRole: VisualRole.Administrator, visualRoleOverride: VisualRole.None } })

  const Badge = () => {
    const { user } = useUserProfile()
    return <output>{user?.visualRole}:{user?.visualRoleOverride || "default"}</output>
  }
  const Profile = () => {
    const [user, setUser] = React.useState({ ...actor, id: userId })
    return (
      <UserStandingProvider>
        <UserProfileProvider userId={userId} user={user} embedded>
          <Badge />
          <UserProfileDetails onVisualRoleChange={(updated) => setUser((current) => ({ ...current, ...updated }))} />
        </UserProfileProvider>
      </UserStandingProvider>
    )
  }
  render(<Profile />)
  fireEvent.click(screen.getByRole("button", { name: "User Details" }))
  const selector = document.getElementById("input-visualRole")!

  fireEvent.change(selector, { target: { value: VisualRole.Sherpa } })
  await screen.findByText("Badge change rejected.")
  expect(selector).toHaveValue(VisualRole.None)
  expect(screen.getByText("Administrator:default")).toBeVisible()

  fireEvent.change(selector, { target: { value: VisualRole.Sherpa } })
  await screen.findByText("Sherpa:Sherpa")
  expect(selector).toHaveValue(VisualRole.Sherpa)
  expect(Fider.session.user.visualRole).toBe(userId === actor.id ? VisualRole.Sherpa : VisualRole.Administrator)
  expect(Fider.session.user.visualRoleOverride).toBe(userId === actor.id ? VisualRole.Sherpa : VisualRole.None)
  expect(screen.queryByText("Badge change rejected.")).toBeNull()

  fireEvent.change(selector, { target: { value: VisualRole.None } })
  await screen.findByText("Administrator:default")
  expect(selector).toHaveValue(VisualRole.None)
  expect(Fider.session.user.visualRoleOverride).toBe(VisualRole.None)
  expect(save).toHaveBeenCalledTimes(3)
})
