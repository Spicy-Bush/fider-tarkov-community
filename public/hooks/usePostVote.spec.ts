import { act, renderHook } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { Post } from "@fider/models"
import * as actions from "@fider/services/actions/post"
import { analytics } from "@fider/services/analytics"
import * as notify from "@fider/services/notify"
import { useFider } from "@fider/hooks/use-fider"
import { RequestError } from "@fider/services/http"
import { usePostVote } from "./usePostVote"

jest.mock("@fider/hooks/use-fider", () => ({ useFider: jest.fn() }))
jest.mock("@fider/services/actions/post", () => ({ setVote: jest.fn() }))
jest.mock("@fider/services/analytics", () => ({ analytics: { event: jest.fn() } }))
jest.mock("@fider/services/notify", () => ({ error: jest.fn() }))

const post = { permissions: { vote: true }, id: 1, number: 7, status: "open", voteType: 0, voteRevision: 0, upvotes: 3, downvotes: 1 } as Post

function deferNextVote() {
  let finish!: (value: Awaited<ReturnType<typeof actions.setVote>>) => void
  const response = new Promise<Awaited<ReturnType<typeof actions.setVote>>>((resolve) => {
    finish = resolve
  })

  jest.mocked(actions.setVote).mockReturnValueOnce(response)
  return finish
}

beforeEach(() => {
  jest.resetAllMocks()
  jest.mocked(useFider).mockReturnValue({
    session: { isAuthenticated: true, user: { id: 1 }, tenant: { id: 1 } }, isReadOnly: false,
  } as ReturnType<typeof useFider>)
})

test("anonymous voting opens sign-in without writing", async () => {
  jest.mocked(useFider).mockReturnValue({
    session: { isAuthenticated: false, tenant: { id: 1 } }, isReadOnly: false,
  } as ReturnType<typeof useFider>)
  const { result } = renderHook(() => usePostVote(post))
  await act(() => result.current.chooseVote("up"))
  expect(result.current.isSignInModalOpen).toBe(true)
  expect(actions.setVote).not.toHaveBeenCalled()
  act(() => result.current.closeSignInModal())
  expect(result.current.isSignInModalOpen).toBe(false)
})

test("a denied vote capability prevents voting on an otherwise open post", async () => {
  const { result } = renderHook(() => usePostVote({ ...post, permissions: { ...post.permissions, vote: false } }))
  expect(result.current.isDisabled).toBe(true)
  await act(() => result.current.chooseVote("up"))
  expect(actions.setVote).not.toHaveBeenCalled()
})

test("one request shows the chosen vote at once, then publishes the returned vote and counts together", async () => {
  const finish = deferNextVote()
  const { result } = renderHook(() => usePostVote(post))
  let sending: Promise<void>

  act(() => {
    sending = result.current.chooseVote("up")
  })

  expect(actions.setVote).toHaveBeenCalledTimes(1)
  expect(actions.setVote).toHaveBeenCalledWith(7, 1, 0)
  expect(result.current).toMatchObject({ voteType: "up", upvotes: 4, downvotes: 1, isSaving: true, isDisabled: false })

  await act(async () => {
    finish({ ok: true, data: { direction: 1, upvotes: 9, downvotes: 2, revision: 1, applied: true } })
    await sending
  })

  expect(result.current).toMatchObject({ voteType: "up", upvotes: 9, downvotes: 2, isSaving: false, isDisabled: false })
})

test.each([
  ["opposite choices", ["up", "down"], -1, 2],
  ["removing a pending vote", ["up", "up"], 0, 2],
  ["returning to the first choice", ["up", "down", "up"], 1, 1],
  ["a hundred alternating choices", Array.from({ length: 100 }, (_, index) => index % 2 ? "down" : "up"), -1, 2],
] as const)("%s preserve the final choice with bounded writes", async (_description, choices, expected, writes) => {
  const finish = deferNextVote()
  jest.mocked(actions.setVote).mockResolvedValue({
    ok: true,
    data: { direction: expected, revision: 2, upvotes: 3, downvotes: expected === -1 ? 2 : 1, applied: true },
  })

  const { result } = renderHook(() => usePostVote(post))
  let sending: Promise<void>

  act(() => {
    for (const choice of choices) {
      sending = result.current.chooseVote(choice as "up" | "down")
    }
  })

  expect(actions.setVote).toHaveBeenCalledTimes(1)
  expect(result.current.voteType).toBe(expected === 1 ? "up" : expected === -1 ? "down" : "none")

  await act(async () => {
    finish({ ok: true, data: { direction: 1, revision: 1, upvotes: 4, downvotes: 1, applied: true } })
    await sending
  })

  expect(actions.setVote).toHaveBeenCalledTimes(writes)
  if (writes === 2) {
    expect(actions.setVote).toHaveBeenLastCalledWith(7, expected, 1)
  }

  expect(result.current).toMatchObject({ voteType: expected === 1 ? "up" : expected === -1 ? "down" : "none", isSaving: false })
  expect(analytics.event).toHaveBeenCalledTimes(writes)
})

test("a newer choice can use the revision returned by a conflicting older request", async () => {
  const finish = deferNextVote()
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: true,
    data: { direction: -1, revision: 8, upvotes: 3, downvotes: 2, applied: true },
  })

  const { result } = renderHook(() => usePostVote(post))
  let sending: Promise<void>

  act(() => {
    sending = result.current.chooseVote("up")
    void result.current.chooseVote("down")
  })

  await act(async () => {
    finish({ ok: true, data: { direction: 0, revision: 7, upvotes: 3, downvotes: 1, applied: false } })
    await sending
  })

  expect(actions.setVote).toHaveBeenLastCalledWith(7, -1, 7)
  expect(result.current.voteType).toBe("down")
  expect(notify.error).not.toHaveBeenCalled()
})

test("an accepted sequence finishes after unmount without retaining a detached control", async () => {
  const finish = deferNextVote()
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: true,
    data: { direction: -1, revision: 2, upvotes: 3, downvotes: 2, applied: true },
  })

  const { result, unmount } = renderHook(() => usePostVote(post))
  let sending: Promise<void>

  act(() => {
    sending = result.current.chooseVote("up")
    void result.current.chooseVote("down")
  })

  unmount()

  await act(async () => {
    finish({ ok: true, data: { direction: 1, revision: 1, upvotes: 4, downvotes: 1, applied: true } })
    await sending
  })

  expect(actions.setVote).toHaveBeenLastCalledWith(7, -1, 1)
})

test("the previous post cannot replace a new post's vote", async () => {
  const finish = deferNextVote()
  const { result, rerender } = renderHook((current) => usePostVote(current), { initialProps: post })
  let sending: Promise<void>

  act(() => {
    sending = result.current.chooseVote("up")
  })

  rerender({ ...post, id: 2, number: 8, upvotes: 20 })

  await act(async () => {
    finish({ ok: true, data: { direction: 1, revision: 1, upvotes: 4, downvotes: 1, applied: true } })
    await sending
  })

  expect(result.current).toMatchObject({ voteType: "none", upvotes: 20, isSaving: false })
})

test("older props cannot roll back a confirmed vote", async () => {
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: true,
    data: { direction: 1, revision: 1, upvotes: 9, downvotes: 2, applied: true },
  })

  const { result, rerender } = renderHook((current) => usePostVote(current), { initialProps: post })
  await act(() => result.current.chooseVote("up"))
  rerender({ ...post, upvotes: 9, downvotes: 2 })

  expect(result.current).toMatchObject({ voteType: "up", upvotes: 9, downvotes: 2 })
})

test("newer server props win over a late acknowledgement", async () => {
  const finish = deferNextVote()
  const { result, rerender } = renderHook((current) => usePostVote(current), { initialProps: post })
  let sending: Promise<void>

  act(() => {
    sending = result.current.chooseVote("up")
  })

  rerender({ ...post, voteType: -1, voteRevision: 8, downvotes: 2 })

  await act(async () => {
    finish({ ok: true, data: { direction: 1, revision: 1, upvotes: 4, downvotes: 1, applied: true } })
    await sending
  })

  expect(result.current).toMatchObject({ voteType: "down", upvotes: 3, downvotes: 2, isSaving: false })
  expect(analytics.event).not.toHaveBeenCalled()
})

test("queued choices do not continue under a different signed-in user", async () => {
  const finish = deferNextVote()
  const fider = {
    session: { isAuthenticated: true, user: { id: 1 }, tenant: { id: 1 } },
    isReadOnly: false,
  } as ReturnType<typeof useFider>
  jest.mocked(useFider).mockReturnValue(fider)
  const { result } = renderHook(() => usePostVote(post))
  let sending: Promise<void>

  act(() => {
    sending = result.current.chooseVote("up")
    void result.current.chooseVote("down")
  })

  fider.session.user.id = 2

  await act(async () => {
    finish({ ok: true, data: { direction: 1, revision: 1, upvotes: 4, downvotes: 1, applied: true } })
    await sending
  })

  expect(actions.setVote).toHaveBeenCalledTimes(1)
  expect(analytics.event).not.toHaveBeenCalled()
})

test.each([
  ["user", 2, 1],
  ["tenant", 1, 2],
])("an uncertain request cannot migrate to a different %s", async (_changed, userID, tenantID) => {
  jest.mocked(actions.setVote).mockResolvedValueOnce({ ok: false, unconfirmed: true, error: { cause: new RequestError("POST", "/vote", "transport", new Error("offline")) } })
  const { result, rerender } = renderHook(() => usePostVote(post))
  await act(() => result.current.chooseVote("up"))

  jest.mocked(useFider).mockReturnValue({
    session: { isAuthenticated: true, user: { id: userID }, tenant: { id: tenantID } },
    isReadOnly: false,
  } as ReturnType<typeof useFider>)
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: true,
    data: { direction: -1, revision: 1, upvotes: 3, downvotes: 2, applied: true },
  })
  rerender()

  await act(() => result.current.chooseVote("down"))

  expect(actions.setVote).toHaveBeenCalledTimes(2)
  expect(actions.setVote).toHaveBeenLastCalledWith(7, -1, 0)
  expect(result.current.voteType).toBe("down")
})

test("an uncertain response releases the control and preserves the confirmed vote", async () => {
  jest.mocked(actions.setVote).mockResolvedValueOnce({ ok: false, unconfirmed: true, error: { cause: new RequestError("PUT", "/vote", "transport", new Error("offline")) } })
  const { result } = renderHook(() => usePostVote(post))
  await act(() => result.current.chooseVote("up"))
  expect(result.current).toMatchObject({ voteType: "none", upvotes: 3, downvotes: 1, isSaving: false, isDisabled: false })
  expect(analytics.event).not.toHaveBeenCalled()
})

test("a different choice after an uncertain result first resolves the original revision", async () => {
  jest.mocked(actions.setVote).mockResolvedValueOnce({ ok: false, unconfirmed: true, error: { cause: new RequestError("POST", "/vote", "transport", new Error("offline")) } })
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: true,
    data: { direction: 1, revision: 1, upvotes: 4, downvotes: 1, applied: false },
  })
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: true,
    data: { direction: -1, revision: 2, upvotes: 3, downvotes: 2, applied: true },
  })

  const { result } = renderHook(() => usePostVote(post))
  await act(() => result.current.chooseVote("up"))
  await act(() => result.current.chooseVote("down"))

  expect(actions.setVote).toHaveBeenNthCalledWith(1, 7, 1, 0)
  expect(actions.setVote).toHaveBeenNthCalledWith(2, 7, 1, 0)
  expect(actions.setVote).toHaveBeenNthCalledWith(3, 7, -1, 1)
  expect(result.current).toMatchObject({ voteType: "down", upvotes: 3, downvotes: 2, isSaving: false })
  expect(analytics.event).toHaveBeenCalledTimes(2)
})

test("a definitive rejection rolls back the display and permits a fresh choice", async () => {
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: false,
    status: 403,
    error: { errors: [{ message: "Post locked" }] },
  })
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: true,
    data: { direction: -1, revision: 1, upvotes: 3, downvotes: 2, applied: true },
  })

  const { result } = renderHook(() => usePostVote(post))
  await act(() => result.current.chooseVote("up"))

  expect(result.current).toMatchObject({ voteType: "none", upvotes: 3, downvotes: 1, isSaving: false })
  expect(notify.error).toHaveBeenCalledWith("Post locked")
  expect(analytics.event).not.toHaveBeenCalled()

  await act(() => result.current.chooseVote("down"))

  expect(actions.setVote).toHaveBeenCalledTimes(2)
  expect(actions.setVote).toHaveBeenLastCalledWith(7, -1, 0)
  expect(result.current.voteType).toBe("down")
})

test("a vote the server did not apply shows the server's state, not the optimistic one", async () => {
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: true,
    data: { direction: -1, revision: 2, upvotes: 3, downvotes: 2, applied: false },
  })
  const { result } = renderHook(() => usePostVote(post))
  await act(() => result.current.chooseVote("up"))
  expect(result.current).toMatchObject({ voteType: "down", upvotes: 3, downvotes: 2, isSaving: false })
})

const optimisticCases: [number, number, number, number][] = [
  [0, 1, 4, 1],
  [0, -1, 3, 2],
  [1, 0, 2, 1],
  [1, -1, 2, 2],
  [-1, 1, 4, 0],
]
test.each(optimisticCases)("optimistic counts: vote %s -> %s shows %s up / %s down", (before, desired, up, down) => {
  jest.mocked(actions.setVote).mockImplementation(() => new Promise(() => {}))
  const { result } = renderHook(() => usePostVote({ ...post, voteType: before }))
  act(() => { void result.current.chooseVote(desired === 0 ? (before === 1 ? "up" : "down") : desired === 1 ? "up" : "down") })
  expect(result.current).toMatchObject({ upvotes: up, downvotes: down })
})

test.each([
  [0, "up", 1, true, "upvote"],
  [0, "down", -1, true, "downvote"],
  [1, "up", 0, true, "unvote"],
  [-1, "down", 0, true, "unvote"],
  [1, "down", -1, true, "toggle-vote"],
  [-1, "up", 1, true, "toggle-vote"],
  [0, "up", 1, false, "upvote"],
  [0, "down", 1, false, null],
] as const)("vote %s -> %s confirmed as %s (applied: %s) records %s", async (before, choice, direction, applied, event) => {
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: true,
    data: { direction, revision: 1, upvotes: 3, downvotes: 1, applied },
  })
  const { result } = renderHook(() => usePostVote({ ...post, voteType: before }))
  await act(() => result.current.chooseVote(choice))
  if (event === null) {
    expect(analytics.event).not.toHaveBeenCalled()
  } else {
    expect(analytics.event).toHaveBeenCalledTimes(1)
    expect(analytics.event).toHaveBeenCalledWith("post", event)
  }
})

test("two views share pending choices and one request sequence", async () => {
  const finish = deferNextVote()
  jest.mocked(actions.setVote).mockResolvedValueOnce({
    ok: true,
    data: { direction: -1, revision: 2, upvotes: 3, downvotes: 2, applied: true },
  })
  const first = renderHook(() => usePostVote(post))
  const second = renderHook(() => usePostVote(post))
  let sending: Promise<void>

  act(() => {
    sending = first.result.current.chooseVote("up")
  })
  expect(second.result.current.voteType).toBe("up")

  act(() => {
    void second.result.current.chooseVote("down")
  })
  expect(first.result.current.voteType).toBe("down")
  expect(actions.setVote).toHaveBeenCalledTimes(1)
  first.unmount()

  await act(async () => {
    finish({ ok: true, data: { direction: 1, revision: 1, upvotes: 4, downvotes: 1, applied: true } })
    await sending
  })
  expect(actions.setVote).toHaveBeenLastCalledWith(7, -1, 1)
  expect(second.result.current).toMatchObject({ voteType: "down", upvotes: 3, downvotes: 2, isSaving: false })
  expect(analytics.event).toHaveBeenCalledTimes(2)
})

test("a remounted view joins an in-flight record and cannot reset its optimistic choice", async () => {
  const finish = deferNextVote()
  const first = renderHook(() => usePostVote(post))
  let sending: Promise<void>
  act(() => { sending = first.result.current.chooseVote("up") })
  first.unmount()
  const replacement = renderHook(() => usePostVote(post))
  expect(replacement.result.current.voteType).toBe("up")

  await act(async () => {
    finish({ ok: true, data: { direction: 1, revision: 1, upvotes: 4, downvotes: 1, applied: true, lastActivityAt: "2026-09-29T12:30:00Z" } })
    await sending
  })
  expect(replacement.result.current).toMatchObject({ voteType: "up", lastActivityAt: "2026-09-29T12:30:00Z" })
  expect(actions.setVote).toHaveBeenCalledTimes(1)
})

test("revoking a mounted view's capability stops a queued opposite choice", async () => {
  const finish = deferNextVote()
  const first = renderHook(() => usePostVote(post))
  const second = renderHook((current) => usePostVote(current), { initialProps: post })
  let sending: Promise<void>
  act(() => {
    sending = first.result.current.chooseVote("up")
    void second.result.current.chooseVote("down")
  })
  second.rerender({ ...post, permissions: { ...post.permissions, vote: false } })

  await act(async () => {
    finish({ ok: true, data: { direction: 1, revision: 1, upvotes: 4, downvotes: 1, applied: true } })
    await sending
  })
  expect(actions.setVote).toHaveBeenCalledTimes(1)
  expect(first.result.current.isDisabled).toBe(true)
})

test("a later denial cannot discard an earlier unconfirmed vote revision", async () => {
  jest.mocked(actions.setVote)
    .mockResolvedValueOnce({ ok: false, status: 403, unconfirmed: true, error: { errors: [{ message: "Sign in again" }] } })
    .mockResolvedValueOnce({ ok: true, unconfirmed: false, data: { direction: 1, revision: 1, upvotes: 4, downvotes: 1, applied: false } })
    .mockResolvedValueOnce({ ok: true, unconfirmed: false, data: { direction: -1, revision: 2, upvotes: 3, downvotes: 2, applied: true } })
  const view = renderHook(() => usePostVote(post))
  await act(() => view.result.current.chooseVote("up"))
  await act(() => view.result.current.chooseVote("down"))
  expect(jest.mocked(actions.setVote).mock.calls).toEqual([[7, 1, 0], [7, 1, 0], [7, -1, 1]])
})
