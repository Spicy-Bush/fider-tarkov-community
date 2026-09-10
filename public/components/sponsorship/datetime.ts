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
