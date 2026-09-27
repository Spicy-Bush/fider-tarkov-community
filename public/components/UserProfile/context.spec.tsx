import React from "react"
import { act, render, renderHook, screen, waitFor } from "@testing-library/react"
import { expect, jest, test } from "@jest/globals"
import { UserRole, UserStatus } from "@fider/models"
import { Fider, actions } from "@fider/services"
import { RequestError } from "@fider/services/http"
import { UserStandingProvider } from "@fider/contexts/UserStandingContext"
import { UserProfileProvider, useUserProfile } from "./context"
import { UserProfile } from "./UserProfile"

jest.mock("@fider/services/actions")

test.each([7, 8])("profile %i retries both stats and standing after a failed initial read", async (userId) => {
  const user = {
    id: 7,
    name: "Member",
    role: UserRole.Visitor,
    status: UserStatus.Active,
    avatarURL: "",
  }
  Fider.initialize({ user, contextID: "profile", tenant: {}, settings: {} })

  jest.mocked(actions.getUserProfileStanding).mockReset().mockResolvedValue({ ok: true, data: { warnings: [], mutes: [] } })
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
  const user = { id: 7, name: "Member", role: UserRole.Visitor, status: UserStatus.Active, avatarURL: "" }
  Fider.initialize({ user, contextID: "profile", tenant: {}, settings: {} })

  let completeOld: (result: Awaited<ReturnType<typeof actions.getUserProfileStats>>) => void
  jest.mocked(actions.getUserProfileStats).mockReset()
    .mockReturnValueOnce(new Promise((resolve) => {
      completeOld = resolve
    }))
    .mockResolvedValue({ ok: true, data: { posts: 9, comments: 0, votes: 0 } })
  jest.mocked(actions.getUserProfileStanding).mockReset().mockResolvedValue({ ok: true, data: { warnings: [], mutes: [] } })

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
