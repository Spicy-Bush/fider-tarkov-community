import { Fider } from "./fider"

export type ErrorContext = 
  | "eventsource"
  | "pageLoader"
  | "localStorage"
  | "api"
  | "parsing"
  | "clipboard"
  | "file"
  | "react"
  | "unknown"

interface ErrorLogOptions {
  silent?: boolean
  context?: ErrorContext
  metadata?: Record<string, unknown>
}

const formatError = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message
  }
  if (typeof error === "string") {
    return error
  }
  return String(error)
}

const shouldLog = (): boolean => {
  return !Fider.isProduction()
}

export const handleError = (error: unknown, context: string, options: ErrorLogOptions = {}): void => {
  const { silent = false, metadata } = options

  if (!silent && shouldLog()) {
    const metadataStr = metadata ? ` ${JSON.stringify(metadata)}` : ""
    console.error(`[${context}]${metadataStr}`, error)
  }
}

export const handleSilentError = (error: unknown, context: string): void => {
  handleError(error, context, { silent: true })
}

export const tryParseJSON = <T>(json: string, fallback: T): T => {
  try {
    return JSON.parse(json) as T
  } catch {
    return fallback
  }
}

export const tryLocalStorageGet = (key: string): string | null => {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

export const tryLocalStorageSet = (key: string, value: string): boolean => {
  try {
    localStorage.setItem(key, value)
    return true
  } catch {
    return false
  }
}

export const tryLocalStorageRemove = (key: string): boolean => {
  try {
    localStorage.removeItem(key)
    return true
  } catch {
    return false
  }
}

export const errors = {
  handle: handleError,
  handleSilent: handleSilentError,
  tryParseJSON,
  tryLocalStorageGet,
  tryLocalStorageSet,
  tryLocalStorageRemove,
  format: formatError,
}

