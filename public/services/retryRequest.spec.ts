import { RequestError } from "./http"
import { retryRequest } from "./retryRequest"

test.each([400, 401, 403, 409, 429])("a first HTTP %s rejection keeps its diagnostics and is not retried", async (status) => {
  const error = { errors: [{ message: "You are muted." }] }
  const send = jest.fn().mockResolvedValue({ ok: false, status, error })

  const result = await retryRequest(send, { delayMs: 0 })

  expect(result).toMatchObject({ ok: false, status, error, unconfirmed: false })
  expect(send).toHaveBeenCalledTimes(1)
})

test.each([400, 401, 403, 409])("a lost acknowledgement followed by HTTP %s remains uncertain", async (status) => {
  const lost = new RequestError("POST", "/api/posts", "transport", new Error("Lost acknowledgement"))
  const send = jest.fn()
    .mockRejectedValueOnce(lost)
    .mockResolvedValue({ ok: false, status, error: { errors: [{ message: "Current rejection" }] } })

  const result = await retryRequest(send, { delayMs: 0 })

  expect(result).toMatchObject({ ok: false, status, unconfirmed: true })
  expect(result.error?.errors).toEqual([{ message: "Current rejection" }])
  expect(result.error?.cause).toBe(lost)
  expect(send).toHaveBeenCalledTimes(2)
})

test("an accepted receipt resolves all earlier uncertainty", async () => {
  const send = jest.fn()
    .mockResolvedValueOnce({ ok: false, status: 408, error: {} })
    .mockResolvedValueOnce({ ok: false, status: 503, error: {} })
    .mockResolvedValue({ ok: true, data: { id: 21 } })

  expect(await retryRequest(send, { delayMs: 0 })).toEqual({ ok: true, data: { id: 21 }, unconfirmed: false })
  expect(send).toHaveBeenCalledTimes(3)
})

test.each([undefined, 408, 500, 503])("temporary failure %s stops at the requested attempt bound", async (status) => {
  const send = jest.fn().mockResolvedValue({ ok: false, status, error: {} })

  const result = await retryRequest(send, { attempts: 2, delayMs: 0 })

  expect(result).toMatchObject({ ok: false, status, unconfirmed: true })
  expect(send).toHaveBeenCalledTimes(2)
})

test("cancellation during a retry delay sends no further requests", async () => {
  const controller = new AbortController()
  const send = jest.fn().mockResolvedValue({ ok: false, status: 503, error: {} })
  const completion = retryRequest(send, { signal: controller.signal })
  controller.abort()
  await expect(completion).rejects.toBe(controller.signal.reason)
  expect(send).toHaveBeenCalledTimes(1)
})
