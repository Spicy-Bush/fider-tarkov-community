import { useSyncExternalStore } from "react"

export interface CookieChoices {
  analytics: boolean
}

const storageKey = "fider-cookie-consent"
const lifetime = 180 * 24 * 60 * 60 * 1000
const listeners = new Set<() => void>()
let choices: CookieChoices | null = null

function readChoices(): CookieChoices | null {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) || "null")
    if (saved && typeof saved.analytics === "boolean" && saved.expiresAt > Date.now()) {
      return { analytics: saved.analytics }
    }
  } catch {
    // Storage can be unavailable in private browsing.
  }

  return null
}

function publish(value: CookieChoices | null): void {
  choices = value
  for (const listener of listeners) listener()
}

if (typeof window !== "undefined") {
  choices = readChoices()
}

export const cookieConsent = {
  snapshot: () => choices,
  subscribe: (listener: () => void) => {
    listeners.add(listener)
    return () => { listeners.delete(listener) }
  },
  listen: () => {
    const refresh = (event: StorageEvent) => {
      if (event.key === storageKey || event.key === null) publish(readChoices())
    }
    window.addEventListener("storage", refresh)
    return () => window.removeEventListener("storage", refresh)
  },
  save: (value: CookieChoices) => {
    try {
      localStorage.setItem(storageKey, JSON.stringify({ ...value, expiresAt: Date.now() + lifetime }))
    } catch {
      // A full store must not preserve an earlier grant after withdrawal.
      try {
        localStorage.removeItem(storageKey)
      } catch {
        // Storage access can be disabled for the whole visit.
      }
    }

    publish(value)
  },
  open: () => { window.dispatchEvent(new Event("cookie-preferences")) },
}

export function useCookieConsent(): CookieChoices | null {
  return useSyncExternalStore(cookieConsent.subscribe, cookieConsent.snapshot, () => null)
}
