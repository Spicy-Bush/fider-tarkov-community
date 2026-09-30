import { act, renderHook } from "@testing-library/react"
import { webcrypto } from "crypto"
import { useSponsorSave } from "./useSponsorSave"
import { Result } from "@fider/services/http"

jest.mock("@fider/services/notify", () => ({ error: jest.fn() }))

Object.defineProperty(globalThis, "crypto", { value: webcrypto, configurable: true })

test("a pending save cannot be replaced by another row and retry keeps its identity", async () => {
  const send = jest.fn<Promise<Result<number>>, [number, string]>()
    .mockResolvedValueOnce({ ok: false, status: 503, error: {} })
    .mockResolvedValueOnce({ ok: true, data: 10 })
  const saved = jest.fn()
  const { result } = renderHook(() => useSponsorSave(send, saved))

  await act(async () => { await result.current.submit(10) })
  expect(result.current.pending).toBe(10)
  expect(result.current.uncertain).toBe(true)

  await act(async () => { await result.current.submit(20) })
  expect(send).toHaveBeenCalledTimes(1)

  await act(async () => { await result.current.retry() })
  expect(send.mock.calls[1]).toEqual(send.mock.calls[0])
  expect(saved).toHaveBeenCalledWith(10)
  expect(result.current.disabled).toBe(false)
})

test("a rejected save releases its identity and double clicks send only one request", async () => {
  let finish!: (result: Result<number>) => void
  const send = jest.fn<Promise<Result<number>>, [number, string]>()
    .mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    .mockResolvedValueOnce({ ok: true, data: 20 })
  const { result } = renderHook(() => useSponsorSave(send, jest.fn()))
  let first: Promise<void> | undefined

  await act(async () => {
    first = result.current.submit(10)
    await result.current.submit(20)
  })
  expect(send).toHaveBeenCalledTimes(1)

  await act(async () => {
    finish({ ok: false, status: 400, error: {} })
    await first
  })
  await act(async () => { await result.current.retry() })
  expect(send).toHaveBeenCalledTimes(1)

  await act(async () => { await result.current.submit(20) })
  expect(send.mock.calls[1][0]).toBe(20)
  expect(send.mock.calls[1][1]).not.toBe(send.mock.calls[0][1])
})
