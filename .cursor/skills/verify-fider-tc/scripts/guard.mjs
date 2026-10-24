// Loopback guard for verify-fider-tc. Pure (no env, no Playwright) so test-guard.mjs can exercise it.
const HOSTS = new Set(["localhost", "127.0.0.1"])

// Returns the parsed URL, or throws. Accepts only http://(localhost|127.0.0.1):<explicit port>, no userinfo.
export function assertLocalUrl(raw, label = "URL") {
  let u
  try { u = new URL(raw) } catch { throw new Error(`refusing ${label}=${raw}: not a URL`) }
  if (u.protocol !== "http:") throw new Error(`refusing ${label}=${raw}: not http`)
  if (u.username || u.password) throw new Error(`refusing ${label}=${raw}: userinfo not allowed`)
  if (!HOSTS.has(u.hostname)) throw new Error(`refusing ${label}=${raw}: host ${u.hostname} is not loopback`)
  if (!u.port) throw new Error(`refusing ${label}=${raw}: explicit port required`)
  return u
}
