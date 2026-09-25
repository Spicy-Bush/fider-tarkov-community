import { jest, test, expect, beforeEach, afterEach } from "@jest/globals"
import React, { useEffect } from "react"
import { act, render, renderHook, screen } from "@testing-library/react"
import { ProfileReviewStatus, useProfileReview } from "./ProfileReviewStatus"
import { http, Result } from "@fider/services/http"

jest.mock("@fider/services/http")

function ReviewFixture(props: Omit<Parameters<typeof useProfileReview>[0], "enabled"> & { refreshKey: number }) {
  const review = useProfileReview({ ...props, enabled: true })

  useEffect(() => {
    if (props.refreshKey > 0) {
      void review.refresh()
    }
  }, [props.refreshKey, review.refresh])

  return <ProfileReviewStatus {...review} />
}

const saved = {
  ok: true,
  data: { changes: [{ field: "name", state: "pending", value: "Saved name", revision: 1 }], name: "Public name", avatarURL: "old.png" },
}

const get = jest.mocked(http.get)

beforeEach(() => {
  jest.useFakeTimers()
  jest.clearAllMocks()
})
afterEach(() => {
  jest.useRealTimers()
})

test("reload shows the saved proposal; approval updates the public profile and stops polling", async () => {
  get.mockResolvedValueOnce(saved).mockResolvedValue({ ok: true, data: { changes: [], name: "Saved name", avatarURL: "old.png" } })
  const name = jest.fn()
  render(<ReviewFixture refreshKey={0} onNameChanged={name} onAvatarChanged={jest.fn()} />)
  await act(async () => {})
  expect(screen.getByText(/Name “Saved name”.*saved and awaiting/)).toBeTruthy()
  expect(name).toHaveBeenLastCalledWith("Public name")
  await act(async () => {
    jest.advanceTimersByTime(5000)
  })
  expect(name).toHaveBeenLastCalledWith("Saved name")
  expect(screen.queryByText(/awaiting a check/)).toBeNull()
  await act(async () => {
    jest.advanceTimersByTime(15000)
  })
  expect(http.get).toHaveBeenCalledTimes(2)
})

test("a stale status response cannot overwrite a more recent save", async () => {
  let respond: (value: Result<unknown>) => void = () => {}
  get
    .mockReturnValueOnce(
      new Promise((resolve) => {
        respond = resolve
      })
    )
    .mockResolvedValue(saved)
  const name = jest.fn()
  const avatar = jest.fn()
  const view = render(<ReviewFixture refreshKey={0} onNameChanged={name} onAvatarChanged={avatar} />)
  view.rerender(<ReviewFixture refreshKey={1} onNameChanged={name} onAvatarChanged={avatar} />)
  await act(async () => {})
  await act(async () => {
    respond({ ok: true, data: { changes: [], name: "Stale name", avatarURL: "" } })
  })
  expect(name).toHaveBeenCalledTimes(1)
  expect(name).toHaveBeenLastCalledWith("Public name")
})

test("status failure preserves the proposal and recovers without resubmitting", async () => {
  get.mockResolvedValueOnce(saved).mockRejectedValueOnce(new Error("offline")).mockResolvedValue(saved)
  render(<ReviewFixture refreshKey={0} onNameChanged={jest.fn()} onAvatarChanged={jest.fn()} />)
  await act(async () => {})
  await act(async () => {
    jest.advanceTimersByTime(5000)
  })
  expect(screen.getByText(/Retrying automatically/)).toBeTruthy()
  expect(screen.getByText(/Name “Saved name”/)).toBeTruthy()
  await act(async () => {
    jest.advanceTimersByTime(10000)
  })
  expect(screen.queryByText(/Retrying automatically/)).toBeNull()
})

test("a published change stays visibly unreviewed until recovery", async () => {
  get
    .mockResolvedValueOnce({ ...saved, data: { ...saved.data, name: "Saved name", changes: [{ ...saved.data.changes[0], published: true }] } })
    .mockResolvedValue({ ok: true, data: { changes: [], name: "Saved name", avatarURL: "old.png" } })
  render(<ReviewFixture refreshKey={0} onNameChanged={jest.fn()} onAvatarChanged={jest.fn()} />)
  await act(async () => {})
  expect(screen.getByText(/is visible.*check is delayed/)).toBeTruthy()
  await act(async () => {
    jest.advanceTimersByTime(5000)
  })
  expect(screen.queryByText(/check is delayed/)).toBeNull()
})

const pendingAvatar = {
  ok: true,
  data: {
    changes: [{ field: "avatar", state: "running", value: "", revision: 2 }],
    name: "Public name",
    avatarURL: "old.png",
    avatarType: "custom",
  },
}

test("a quick avatar check finishes Save with the published image and no pending notice", async () => {
  get
    .mockResolvedValueOnce(saved)
    .mockResolvedValueOnce(pendingAvatar)
    .mockResolvedValue({
      ok: true,
      data: { changes: [], name: "Public name", avatarURL: "new.png", avatarType: "custom" },
    })
  const avatar = jest.fn()
  const { result } = renderHook(() => useProfileReview({ enabled: true, onNameChanged: jest.fn(), onAvatarChanged: avatar }))
  await act(async () => {})
  const finished = jest.fn()

  await act(async () => {
    void result.current.refresh("avatar").then(finished)
  })

  const view = render(<ProfileReviewStatus {...result.current} />)
  expect(screen.queryByText(/awaiting a check/)).toBeNull()
  expect(finished).not.toHaveBeenCalled()

  await act(async () => {
    jest.advanceTimersByTime(500)
  })

  view.rerender(<ProfileReviewStatus {...result.current} />)
  expect(finished).toHaveBeenCalledTimes(1)
  expect(avatar).toHaveBeenLastCalledWith("new.png", "custom")
  expect(screen.queryByText(/awaiting a check/)).toBeNull()
})

test("a slow check releases Save after two seconds and still publishes later", async () => {
  get.mockResolvedValue(pendingAvatar)
  const avatar = jest.fn()
  const { result } = renderHook(() => useProfileReview({ enabled: true, onNameChanged: jest.fn(), onAvatarChanged: avatar }))
  await act(async () => {})
  const finished = jest.fn()

  await act(async () => {
    void result.current.refresh("avatar").then(finished)
  })

  for (let elapsed = 0; elapsed < 2000; elapsed += 500) {
    await act(async () => {
      jest.advanceTimersByTime(500)
    })
  }

  expect(finished).toHaveBeenCalledTimes(1)
  render(<ProfileReviewStatus {...result.current} />)
  expect(screen.getByText(/Avatar.*saved and awaiting/)).toBeTruthy()

  get.mockResolvedValue({ ok: true, data: { ...pendingAvatar.data, changes: [], avatarURL: "new.png" } })

  await act(async () => {
    jest.advanceTimersByTime(5000)
  })

  expect(avatar).toHaveBeenLastCalledWith("new.png", "custom")
})

test("starting a save prevents an older status request from overwriting its response", async () => {
  let respond: (value: Result<unknown>) => void = () => {}
  get.mockReturnValue(
    new Promise((resolve) => {
      respond = resolve
    })
  )
  const avatar = jest.fn()
  const { result } = renderHook(() => useProfileReview({ enabled: true, onNameChanged: jest.fn(), onAvatarChanged: avatar }))

  act(() => result.current.pause())

  await act(async () => {
    respond(pendingAvatar)
  })

  expect(avatar).not.toHaveBeenCalled()
})

test.each([
  { state: "rejected", published: false, avatarURL: "old.png" },
  { state: "pending", published: true, avatarURL: "new.png" },
])("Save settles when the check is $state and publication is $published", async ({ state, published, avatarURL }) => {
  get.mockResolvedValueOnce(saved).mockResolvedValue({
    ok: true,
    data: { ...pendingAvatar.data, avatarURL, changes: [{ ...pendingAvatar.data.changes[0], state, published }] },
  })
  const avatar = jest.fn()
  const { result } = renderHook(() => useProfileReview({ enabled: true, onNameChanged: jest.fn(), onAvatarChanged: avatar }))
  await act(async () => {})

  await act(async () => {
    await result.current.refresh("avatar")
  })

  expect(result.current.settling).toBe(false)
  expect(avatar).toHaveBeenLastCalledWith(avatarURL, "custom")
  expect(result.current.changes[0].state).toBe(state)
})

test("an unresponsive status request does not keep Save busy indefinitely", async () => {
  get.mockResolvedValueOnce(saved).mockReturnValue(new Promise(() => {}))
  const { result, unmount } = renderHook(() => useProfileReview({ enabled: true, onNameChanged: jest.fn(), onAvatarChanged: jest.fn() }))
  await act(async () => {})
  const finished = jest.fn()

  await act(async () => {
    void result.current.refresh("avatar").then(finished)
    jest.advanceTimersByTime(2000)
  })

  expect(finished).toHaveBeenCalledTimes(1)
  unmount()
  expect(jest.getTimerCount()).toBe(0)
})
