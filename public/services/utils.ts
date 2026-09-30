import { Fider } from "@fider/services/fider"

export const delay = (ms: number) => {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export const classSet = (input?: any): string => {
  let classes = ""
  if (input) {
    for (const key in input) {
      if (key && !!input[key]) {
        classes += ` ${key}`
      }
    }
    return classes.trim()
  }
  return ""
}

type DateFormat = "full" | "short" | "date"
type DateOptsMap = {
  [key in DateFormat]: Intl.DateTimeFormatOptions
}

const dateOpts: DateOptsMap = {
  date: { day: "numeric", month: "short", year: "numeric" },
  short: { month: "short", year: "numeric" },
  full: { day: "2-digit", month: "long", year: "numeric", hour: "numeric", minute: "numeric" },
}

let dateLocale: string | undefined
const dateFormatters = new Map<DateFormat, Intl.DateTimeFormat>()
let relativeTimeFormatter: Intl.RelativeTimeFormat | undefined
let relativeTimeLocale: string | undefined

export const formatDate = (locale: string, input: Date | string, format: DateFormat = "full"): string => {
  const date = input instanceof Date ? input : new Date(input)

  try {
    if (dateLocale !== locale) {
      dateFormatters.clear()
      dateLocale = locale
    }

    let formatter = dateFormatters.get(format)

    if (!formatter) {
      formatter = new Intl.DateTimeFormat(locale, dateOpts[format])
      dateFormatters.set(format, formatter)
    }

    return formatter.format(date)
  } catch {
    return date.toLocaleString(locale)
  }
}

export const timeSince = (locale: string, now: Date, date: Date, dateFormat: DateFormat = "short"): string => {
  try {
    const seconds = Math.round((now.getTime() - date.getTime()) / 1000)
    const minutes = Math.round(seconds / 60)
    const hours = Math.round(minutes / 60)
    const days = Math.round(hours / 24)
    const months = Math.round(days / 30)
    const years = Math.round(days / 365)

    if (!relativeTimeFormatter || relativeTimeLocale !== locale) {
      relativeTimeFormatter = new Intl.RelativeTimeFormat(locale, { numeric: "auto" })
      relativeTimeLocale = locale
    }

    const rtf = relativeTimeFormatter
    return (
      (Math.abs(seconds) < 60 && rtf.format(-1 * seconds, "seconds")) ||
      (Math.abs(minutes) < 60 && rtf.format(-1 * minutes, "minutes")) ||
      (Math.abs(hours) < 24 && rtf.format(-1 * hours, "hours")) ||
      (Math.abs(days) < 30 && rtf.format(-1 * days, "days")) ||
      (Math.abs(days) < 365 && rtf.format(-1 * months, "months")) ||
      rtf.format(-1 * years, "years")
    )
  } catch {
    return formatDate(locale, date, dateFormat)
  }
}
export const fileToBase64 = async (file: File): Promise<string> => {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener(
      "load",
      () => {
        const parts = (reader.result as string).split("base64,")
        resolve(parts[1])
      },
      false
    )

    reader.addEventListener(
      "error",
      () => {
        reject(reader.error)
      },
      false
    )

    reader.readAsDataURL(file)
  })
}

export const timeAgo = (date: string | Date): number => {
  const d = date instanceof Date ? date : new Date(date)
  return (new Date().getTime() - d.getTime()) / 1000
}

export const isCookieEnabled = (): boolean => {
  try {
    document.cookie = "cookietest=1"
    const ret = document.cookie.indexOf("cookietest=") !== -1
    document.cookie = "cookietest=1; expires=Thu, 01-Jan-1970 00:00:01 GMT"
    return ret
  } catch (e) {
    return false
  }
}

export const uploadedImageURL = (bkey: string | undefined, size?: number): string | undefined => {
  if (bkey) {
    if (size) {
      return `${Fider.settings.assetsURL}/static/images/${bkey}?size=${size}`
    }
    return `${Fider.settings.assetsURL}/static/images/${bkey}`
  }
  return undefined
}

export const truncate = (input: string, maxLength: number): string => {
  if (input && input.length > maxLength) {
    return `${input.substr(0, maxLength)}...`
  }
  return input
}

export type StringObject<T = any> = {
  [key: string]: T
}

export const copyToClipboard = async (text: string): Promise<void> => {
  if (window.navigator.clipboard?.writeText) {
    try {
      await window.navigator.clipboard.writeText(text)
      return
    } catch {
      // Browser permissions can restrict the Clipboard API while still allowing a user-initiated copy.
    }
  }

  const previousFocus = document.activeElement
  const input = document.createElement("textarea")
  input.value = text
  input.readOnly = true
  input.style.position = "fixed"
  input.style.opacity = "0"
  document.body.appendChild(input)

  try {
    input.select()

    if (!document.execCommand("copy")) {
      throw new Error("The browser did not allow copying to the clipboard.")
    }
  } finally {
    input.remove()

    if (previousFocus instanceof HTMLElement && previousFocus.isConnected) {
      previousFocus.focus()
    }
  }
}

export const clearUrlHash = (replace?: boolean) => {
  const oldURL = window.location.href
  const newURL = window.location.pathname + window.location.search
  if (replace) {
    window.history.replaceState("", document.title, newURL)
  } else {
    window.history.pushState("", document.title, newURL)
  }
  // Trigger event manually
  const hashChangeEvent = new HashChangeEvent("hashchange", {
    oldURL,
    newURL,
    cancelable: true,
    bubbles: true,
    composed: false,
  })
  if (!window.dispatchEvent(hashChangeEvent)) {
    // Event got cancelled
    window.history.replaceState("", document.title, oldURL)
  }
}
