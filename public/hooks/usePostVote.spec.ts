import { act, renderHook } from "@testing-library/react"
import { beforeEach, expect, test } from "@jest/globals"
import { Post } from "@fider/models"
import { actions, analytics } from "@fider/services"
import { useFider } from "@fider/hooks"
import { RequestError } from "@fider/services/http"
import { usePostVote } from "./usePostVote"

jest.mock("@fider/hooks", () => ({ useFider: jest.fn() }))
jest.mock("@fider/services", () => ({ actions: { setVote: jest.fn() }, analytics: { event: jest.fn() }, notify: { error: jest.fn() } }))

const post = { id: 1, number: 7, status: "open", voteType: 0, voteRevision: 0, upvotes: 3, downvotes: 1 } as Post

beforeEach(() => {
  jest.resetAllMocks()
  jest.mocked(useFider).mockReturnValue({
    session: { isAuthenticated: true }, isReadOnly: false,
  } as ReturnType<typeof useFider>)
})

test("anonymous voting opens sign-in without writing", async () => {
  jest.mocked(useFider).mockReturnValue({
    session: { isAuthenticated: false }, isReadOnly: false,
  } as ReturnType<typeof useFider>)
  const { result } = renderHook(() => usePostVote(post))
  await act(() => result.current.chooseVote("up"))
  expect(result.current.isSignInModalOpen).toBe(true)
  expect(actions.setVote).not.toHaveBeenCalled()
  act(() => result.current.closeSignInModal())
  expect(result.current.isSignInModalOpen).toBe(false)
})

test.each(["completed", "declined", "duplicate", "deleted"])("%s posts cannot be voted on", async (status) => {
  const { result } = renderHook(() => usePostVote({ ...post, status }))
  expect(result.current.isDisabled).toBe(true)
  await act(() => result.current.chooseVote("up"))
  expect(actions.setVote).not.toHaveBeenCalled()
})

test("one request publishes the returned vote and counts together", async () => {
  let finish: (value: Awaited<ReturnType<typeof actions.setVote>>) => void = () => {}
  jest.mocked(actions.setVote).mockImplementation(() => new Promise((resolve) => { finish = resolve }))
  const changed = jest.fn()
  const { result } = renderHook(() => usePostVote(post, changed))
  let sending: Promise<void>
  act(() => {
    sending = result.current.chooseVote("up")
    void result.current.chooseVote("down")
  })
  expect(actions.setVote).toHaveBeenCalledTimes(1)
  expect(actions.setVote).toHaveBeenCalledWith(7, 1, 0)
  expect(result.current.isDisabled).toBe(true)
  await act(async () => {
    finish({ ok: true, data: { direction: 1, upvotes: 4, downvotes: 1, revision: 1, applied: true } })
    await sending
  })
  expect(result.current).toMatchObject({ voteType: "up", upvotes: 4, downvotes: 1, isDisabled: false })
  expect(changed).toHaveBeenCalledWith(4, 1)
})

test("an uncertain response releases the control and preserves the confirmed vote", async () => {
  jest.mocked(actions.setVote).mockRejectedValueOnce(new RequestError("PUT", "/vote", "transport", new Error("offline")))
  const { result } = renderHook(() => usePostVote(post))
  await act(() => result.current.chooseVote("up"))
  expect(result.current).toMatchObject({ voteType: "none", upvotes: 3, downvotes: 1, isDisabled: false })
  expect(analytics.event).not.toHaveBeenCalled()
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
