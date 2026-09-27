import { analytics, notify } from "@fider/services"

export interface ErrorItem {
  field?: string
  message: string
}

export interface Failure {
  errors?: ErrorItem[]
  cause?: unknown
}

export type Result<T = void> =
  | { ok: true; data: T; error?: never; headers?: Headers }
  | { ok: false; error: Failure; data?: never; status?: number; headers?: Headers }

export class RequestError extends Error {
  readonly cause: unknown

  constructor(
    readonly method: string,
    readonly path: string,
    readonly phase: "transport" | "response",
    cause: unknown,
    readonly status?: number
  ) {
    super(`Failed to ${method} ${path} (${phase}${status === undefined ? "" : `, HTTP ${status}`})`)
    this.name = "RequestError"
    Object.defineProperty(this, "cause", { value: cause, enumerable: false })
  }
}

async function toResult<T>(response: Response, method: string, path: string, options?: RequestOptions): Promise<Result<T>> {
  let body: any
  let parseFailure: unknown
  if (response.status !== 204) {
    try {
      body = await response.json()
    } catch (cause) {
      if (response.ok) {
        throw new RequestError(method, path, "response", cause, response.status)
      }
      parseFailure = cause
    }
  }

  if (response.status < 400) {
    return {
      ok: true,
      data: body as T,
      headers: options?.includeHeaders ? response.headers : undefined,
    }
  }

  if (options?.notifyOnError !== false) {
    if (response.status >= 500) {
      notify.error("An unexpected error occurred while processing your request.")
    } else if (response.status === 401) {
      notify.error("You need to be authenticated to perform this operation.")
    } else if (response.status === 403) {
      notify.error("You are not authorized to perform this operation.")
    }
  }

  const failure: Failure = {
    errors: Array.isArray(body?.errors)
      ? body.errors.filter((item: ErrorItem) => item && typeof item.message === "string")
      : [{ message: typeof body?.message === "string" ? body.message : `Request failed (HTTP ${response.status}).` }],
  }
  if (parseFailure !== undefined) {
    Object.defineProperty(failure, "cause", { value: parseFailure, enumerable: false })
  }
  return {
    ok: false,
    status: response.status,
    headers: options?.includeHeaders ? response.headers : undefined,
    error: failure,
  }
}
interface RequestOptions {
  signal?: AbortSignal
  includeHeaders?: boolean
  notifyOnError?: boolean
}

async function request<T>(url: string, method: "GET" | "POST" | "PUT" | "DELETE", body?: any, options?: RequestOptions): Promise<Result<T>> {
  const headers: [string, string][] = [
    ["Accept", "application/json"],
    ["Content-Type", "application/json"],
  ]
  const encodedBody = JSON.stringify(body)
  const path = url.split(/[?#]/, 1)[0]
  let response: Response
  try {
    response = await fetch(url, {
      method,
      headers,
      body: encodedBody,
      credentials: "same-origin",
      signal: options?.signal,
    })
  } catch (cause) {
    throw new RequestError(method, path, "transport", cause)
  }
  return toResult<T>(response, method, path, options)
}

export const http = {
  get: async <T = void>(url: string, options?: RequestOptions): Promise<Result<T>> => {
    return await request<T>(url, "GET", undefined, options)
  },
  getWithHeaders: async <T = void>(url: string): Promise<Result<T>> => {
    return await request<T>(url, "GET", undefined, { includeHeaders: true })
  },
  post: async <T = void>(url: string, body?: any, options?: RequestOptions): Promise<Result<T>> => {
    return await request<T>(url, "POST", body, options)
  },
  put: async <T = void>(url: string, body?: any, options?: RequestOptions): Promise<Result<T>> => {
    return await request<T>(url, "PUT", body, options)
  },
  delete: async <T = void>(url: string, body?: any, options?: RequestOptions): Promise<Result<T>> => {
    return await request<T>(url, "DELETE", body, options)
  },
  event:
    (category: string, action: string) =>
    <T>(result: Result<T>): Result<T> => {
      if (result && result.ok) {
        analytics.event(category, action)
      }
      return result
    },
}
