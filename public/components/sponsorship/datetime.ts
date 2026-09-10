/** Format a UTC ISO instant for <input type="datetime-local"> in the browser's local zone. */
export function utcToDatetimeLocalValue(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ""
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** Parse datetime-local (local wall time) to UTC ISO for the API. */
export function datetimeLocalToUtcIso(localValue: string): string {
  const d = new Date(localValue)
  if (Number.isNaN(d.getTime())) {
    throw new Error("invalid datetime")
  }
  return d.toISOString()
}

/** Short IANA-ish label for the browser local timezone (e.g. "AEDT", "GMT+10"). */
export function browserTimeZoneLabel(now = new Date()): string {
  try {
    const parts = new Intl.DateTimeFormat(undefined, { timeZoneName: "short" }).formatToParts(now)
    const tz = parts.find((p) => p.type === "timeZoneName")?.value
    if (tz) return tz
  } catch {
    /* ignore */
  }
  const offsetMin = -now.getTimezoneOffset()
  const sign = offsetMin >= 0 ? "+" : "-"
  const abs = Math.abs(offsetMin)
  const hh = String(Math.floor(abs / 60)).padStart(2, "0")
  const mm = String(abs % 60).padStart(2, "0")
  return `UTC${sign}${hh}:${mm}`
}
