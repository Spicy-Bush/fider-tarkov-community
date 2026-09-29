import { noSessionPermissions, noUserPermissions } from "@fider/services/testing/permissions"
import React, { useState } from "react"
import { act, fireEvent, render, renderHook, screen } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { Fider, ServerData } from "@fider/services/fider"
import * as actions from "@fider/services/actions/user"
import { RequestError } from "@fider/services/http"
import { UserStandingProvider, useUserStanding } from "@fider/contexts/UserStandingContext"
import { useCurrentUser, useFider } from "./use-fider"

jest.mock("@fider/services/actions/user")

const initial = {
  permissions: { ...noSessionPermissions, createPosts: true },
  title: "Home",
  page: "Home/Home.page",
  contextID: "home",
  props: {},
  settings: { locale: "en", environment: "development" },
  tenant: { id: 1, name: "Before", status: 1 },
  user: { permissions: noUserPermissions, id: 7, role: "visitor", isAdministrator: false, isMuted: false },
} as ServerData

beforeEach(() => {
  jest.restoreAllMocks()
  Fider.initialize(initial)
  jest.mocked(actions.getUserProfileStanding).mockReset().mockResolvedValue({ ok: true, data: { sessionPermissions: noSessionPermissions, warnings: [], mutes: [] } })
})

function Editor() {
  const fider = useFider()
  const user = useCurrentUser()
  const [draft, setDraft] = useState("")

  return (
    <>
      <output>{fider.session.tenant.name}:{user?.role}:{String(fider.isReadOnly)}</output>
      <input aria-label="Draft" value={draft} onChange={(event) => setDraft(event.target.value)} />
    </>
  )
}

test("fresh server data updates existing consumers without discarding their drafts", () => {
  render(<Editor />)
  fireEvent.change(screen.getByLabelText("Draft"), { target: { value: "An unfinished reply" } })
  const session = Fider.session

  act(() => Fider.refresh({
    ...initial,
    contextID: "post",
    tenant: { ...initial.tenant, name: "After", status: 3 },
    user: { ...initial.user!, role: "administrator", isAdministrator: true },
  } as ServerData))

  expect(screen.getByText("After:administrator:true")).toBeVisible()
  expect(screen.getByLabelText("Draft")).toHaveValue("An unfinished reply")
  expect(Fider.session).toBe(session)
  expect(session.contextID).toBe("post")
})

test("anonymous consumers receive tenant and settings changes", () => {
  Fider.initialize({ ...initial, user: undefined })
  const { result } = renderHook(() => {
    const fider = useFider()
    return { name: fider.session.tenant.name, locale: fider.settings.locale, user: useCurrentUser() }
  })

  act(() => Fider.refresh({
    ...initial,
    user: undefined,
    tenant: { ...initial.tenant, name: "After" },
    settings: { ...initial.settings, locale: "de" },
  }))

  expect(result.current).toEqual({ name: "After", locale: "de", user: undefined })
})

test("standing refresh publishes server permissions without deriving them from role or mute", () => {
  const { result } = renderHook(() => useFider().session.permissions)
  expect(result.current.createPosts).toBe(true)

  act(() => Fider.session.updateUserStanding({
    warnings: [],
    mutes: [],
    sessionPermissions: { ...noSessionPermissions, manageQueue: true },
  }))

  expect(Fider.session.user.role).toBe("visitor")
  expect(Fider.session.user.isMuted).toBe(false)
  expect(result.current.createPosts).toBe(false)
  expect(result.current.manageQueue).toBe(true)
})

test("standing follows fresh server mute status and ignores details from an older page", async () => {
  const { result } = renderHook(useUserStanding, { wrapper: UserStandingProvider })
  const oldPage = result.current.refetch
  const mute = { id: 1, reason: "Review", createdAt: "2026-01-01T00:00:00Z", isActive: true }

  jest.mocked(actions.getUserProfileStanding).mockResolvedValueOnce({ ok: true, data: { sessionPermissions: noSessionPermissions, warnings: [], mutes: [mute] } })
  await act(() => result.current.refetch())
  expect(result.current).toMatchObject({ isMuted: true, muteReason: "Review" })
  expect(Fider.session.user.isMuted).toBe(true)

  act(() => Fider.refresh({ ...initial, contextID: "next" }))
  expect(result.current).toMatchObject({ isMuted: false, muteReason: "", mutes: [] })

  await act(() => oldPage())
  expect(result.current.isMuted).toBe(false)
  expect(Fider.session.user.isMuted).toBe(false)

  act(() => Fider.refresh({ ...initial, contextID: "muted", user: { ...initial.user!, isMuted: true } }))
  expect(result.current.isMuted).toBe(true)
})

test("standing uses server activity despite clock skew and recovers when a mute is removed", async () => {
  const clock = jest.spyOn(Date, "now").mockReturnValue(Date.parse("2030-01-01T00:00:00Z"))
  Fider.refresh({ ...initial, user: { ...initial.user!, isMuted: true, latestMuteId: 1 } })
  const { result } = renderHook(useUserStanding, { wrapper: UserStandingProvider })
  const mute = {
    id: 1,
    reason: "Server-active mute",
    createdAt: "2026-01-01T00:00:00Z",
    expiresAt: "2027-01-01T00:00:00Z",
    isActive: true,
  }

  try {
    jest.mocked(actions.getUserProfileStanding).mockResolvedValueOnce({ ok: true, data: { sessionPermissions: noSessionPermissions, warnings: [], mutes: [mute] } })
    await act(() => result.current.refetch())
    expect(result.current.isMuted).toBe(true)
    expect(result.current.muteReason).toBe("Server-active mute")

    await act(() => result.current.refetch())
    expect(result.current.isMuted).toBe(false)
    expect(Fider.session.user.isMuted).toBe(false)
  } finally {
    clock.mockRestore()
  }
})

test("standing publishes active warnings and mutes together, then clears both when removed", async () => {
  const warning = { id: 2, reason: "Warning", createdAt: "2026-01-01T00:00:00Z", isActive: true }
  const mute = { ...warning, id: 3, reason: "Mute" }
  jest.mocked(actions.getUserProfileStanding).mockResolvedValueOnce({ ok: true, data: { sessionPermissions: noSessionPermissions, warnings: [warning], mutes: [mute] } })
  const { result } = renderHook(() => ({ standing: useUserStanding(), user: useCurrentUser() }), { wrapper: UserStandingProvider })

  await act(() => result.current.standing.refetch())

  expect(result.current.user).toMatchObject({ hasWarning: true, latestWarningId: 2, isMuted: true, latestMuteId: 3 })
  expect(result.current.standing.warnings).toEqual([warning])

  await act(() => result.current.standing.refetch())

  expect(result.current.user).toMatchObject({ hasWarning: false, isMuted: false })
  expect(result.current.user?.latestWarningId).toBeUndefined()
  expect(result.current.user?.latestMuteId).toBeUndefined()
})

test("an older standing response cannot overwrite a newer refresh on the same page", async () => {
  let resolveFirst: (result: Awaited<ReturnType<typeof actions.getUserProfileStanding>>) => void
  jest.mocked(actions.getUserProfileStanding).mockReturnValueOnce(new Promise((resolve) => {
    resolveFirst = resolve
  }))
  const { result } = renderHook(useUserStanding, { wrapper: UserStandingProvider })
  let first: Promise<void>

  act(() => {
    first = result.current.refetch()
  })
  const firstSignal = jest.mocked(actions.getUserProfileStanding).mock.calls[0][1]!
  await act(() => result.current.refetch())

  expect(firstSignal.aborted).toBe(true)
  await act(async () => {
    resolveFirst!({ ok: true, data: {
      sessionPermissions: noSessionPermissions,
      warnings: [],
      mutes: [{ id: 9, reason: "Old response", createdAt: "2026-01-01T00:00:00Z", isActive: true }],
    } })
    await first
  })

  expect(result.current.isMuted).toBe(false)
  expect(result.current.mutes).toEqual([])
  expect(Fider.session.user.isMuted).toBe(false)
})

test("failed standing refresh preserves the server mute and retries successfully", async () => {
  Fider.refresh({ ...initial, user: { ...initial.user!, isMuted: true } })
  jest.mocked(actions.getUserProfileStanding).mockRejectedValueOnce(new RequestError("GET", "/api/user/profile/7/standing", "transport", new Error("Disconnected")))
  const { result } = renderHook(useUserStanding, { wrapper: UserStandingProvider })

  await act(() => result.current.refetch())
  expect(result.current).toMatchObject({ isMuted: true, isLoading: false, error: "Could not load account standing." })

  await act(() => result.current.refetch())
  expect(result.current).toMatchObject({ isMuted: false, isLoading: false, error: null })
})

test("standing refresh does not disguise programming defects as request failures", async () => {
  const defect = new Error("Invalid standing implementation")
  jest.mocked(actions.getUserProfileStanding).mockRejectedValueOnce(defect)
  const { result } = renderHook(useUserStanding, { wrapper: UserStandingProvider })

  await act(async () => {
    await expect(result.current.refetch()).rejects.toBe(defect)
  })

  expect(result.current).toMatchObject({ isLoading: false, error: null })
})
