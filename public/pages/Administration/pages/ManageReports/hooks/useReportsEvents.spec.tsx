import { act, renderHook } from "@testing-library/react"
import { afterEach, beforeEach, expect, test } from "@jest/globals"
import { reportsEventSource } from "@fider/services"
import { useReportsEvents } from "./useReportsEvents"

jest.mock("@fider/services", () => ({
  reportsEventSource: { connect: jest.fn(), disconnect: jest.fn(), on: jest.fn() },
}))

const listeners = new Map<string, () => void>()

beforeEach(() => {
  jest.clearAllMocks()
  jest.useFakeTimers()
  listeners.clear()
  jest.mocked(reportsEventSource.on).mockImplementation((name, listener) => {
    listeners.set(name, listener as () => void)
    return () => {
      listeners.delete(name)
    }
  })
})

afterEach(() => jest.useRealTimers())

test("events coalesce while a refresh is running and retain a later refresh", async () => {
  let finish!: () => void
  const pending = new Promise<void>((resolve) => {
    finish = resolve
  })
  const refresh = jest.fn<Promise<void>, []>().mockReturnValueOnce(pending).mockResolvedValue(undefined)
  const { unmount } = renderHook(() => useReportsEvents(refresh))

  act(() => {
    listeners.get("reports.changed")!()
    listeners.get("reports.changed")!()
    jest.advanceTimersByTime(100)
  })
  expect(refresh).toHaveBeenCalledTimes(1)

  act(() => {
    listeners.get("connection.open")!()
    jest.advanceTimersByTime(100)
    listeners.get("reports.changed")!()
    jest.advanceTimersByTime(100)
  })
  expect(refresh).toHaveBeenCalledTimes(1)

  await act(async () => {
    finish()
    await pending
  })
  await act(async () => {
    jest.advanceTimersByTime(100)
  })
  expect(refresh).toHaveBeenCalledTimes(2)

  act(() => listeners.get("reports.changed")!())
  unmount()
  act(() => jest.advanceTimersByTime(100))
  expect(refresh).toHaveBeenCalledTimes(2)
  expect(listeners.size).toBe(0)
  expect(reportsEventSource.disconnect).toHaveBeenCalledTimes(1)
})
