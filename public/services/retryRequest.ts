import { RequestError, Result, requestOutcome } from "./http"

export type RetryResult<T> = Result<T> & { unconfirmed: boolean }

export async function retryRequest<T>(
  send: () => Promise<Result<T>>,
  options: { attempts?: number; delayMs?: number; signal?: AbortSignal } = {}
): Promise<RetryResult<T>> {
  const attempts = options.attempts ?? 3
  const delayMs = options.delayMs ?? 500
  let unconfirmed = false
  let uncertainCause: unknown

  for (let attempt = 0; ; attempt++) {
    if (options.signal?.aborted) {
      throw options.signal.reason
    }
    let result: Result<T>

    try {
      result = await send()
    } catch (cause) {
      if (!(cause instanceof RequestError)) {
        throw cause
      }

      result = {
        ok: false,
        error: { errors: [{ message: "The request could not be sent! Please check your internet connection." }], cause },
      }
    }

    if (result.ok) {
      return { ...result, unconfirmed: false }
    }

    const outcome = requestOutcome(result)
    if (outcome === "uncertain") {
      unconfirmed = true
      uncertainCause ??= result.error.cause ?? result.error
    }

    if (outcome === "rejected" || attempt + 1 === attempts) {
      return {
        ...result,
        error: { ...result.error, cause: result.error.cause ?? uncertainCause },
        unconfirmed,
      }
    }

    await new Promise<void>((resolve, reject) => {
      const abort = () => {
        clearTimeout(timer)
        options.signal?.removeEventListener("abort", abort)
        reject(options.signal?.reason)
      }
      const timer = setTimeout(() => {
        options.signal?.removeEventListener("abort", abort)
        resolve()
      }, delayMs * (attempt + 1))

      options.signal?.addEventListener("abort", abort, { once: true })
      if (options.signal?.aborted) {
        abort()
      }
    })
  }
}