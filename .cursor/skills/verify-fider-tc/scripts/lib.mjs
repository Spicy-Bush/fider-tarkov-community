// Shared helpers for verify-fider-tc drive scripts. Imported by seed.mjs and drive-*.mjs.
import { chromium } from "@playwright/test"
import { execSync } from "node:child_process"
import fs from "node:fs"
import path from "node:path"

export const URL_ = process.env.FIDER_URL
export const MH = process.env.MAILHOG_API
export const RUN = process.env.FIDER_RUN_DIR
export const EVID = process.env.EVIDENCE_DIR
export const ADMIN = process.env.ADMIN_EMAIL
if (!URL_ || !URL_.startsWith("http://localhost:")) throw new Error(`refusing non-local FIDER_URL=${URL_}`)

export const sql = (q) => execSync(`${process.env.PSQL} -c ${JSON.stringify(q)}`, { encoding: "utf8" }).trim()

// Latest magic link MailHog received for `to`, matching `pathPart` (e.g. "/signup/verify", "/signin/verify").
export async function latestLink(to, pathPart, tries = 20) {
  for (let i = 0; i < tries; i++) {
    const r = await fetch(`${MH}/api/v2/search?kind=to&query=${encodeURIComponent(to)}`)
    const j = await r.json()
    for (const m of j.items || []) {
      const body = m.Content.Body.replace(/=\r?\n/g, "").replace(/=3D/gi, "=")
      const re = new RegExp(`https?://[^"'\\s<>]*${pathPart.replace(/\//g, "\\/")}\\?k=[A-Za-z0-9_-]+`)
      const hit = body.match(re)
      if (hit) return hit[0]
    }
    await new Promise((r) => setTimeout(r, 500))
  }
  throw new Error(`no ${pathPart} link mailed to ${to}`)
}

export async function launch({ admin = true, locale } = {}) {
  const browser = await chromium.launch({ headless: true })
  const opts = { viewport: { width: 1280, height: 900 } }
  if (admin) opts.storageState = path.join(RUN, "admin-state.json")
  if (locale) opts.locale = locale
  const context = await browser.newContext(opts)
  const page = await context.newPage()
  return { browser, context, page }
}

export async function serverData(page) {
  return JSON.parse(await page.locator("#server-data").textContent())
}

export function evidence(feature) {
  const dir = path.join(EVID, feature)
  fs.mkdirSync(dir, { recursive: true })
  return {
    dir,
    async shot(page, name) { await page.screenshot({ path: path.join(dir, `${name}.png`), fullPage: true }) },
    async aria(page, name) { fs.writeFileSync(path.join(dir, `${name}.aria.txt`), await page.locator("body").ariaSnapshot()) },
    write(name, data) { fs.writeFileSync(path.join(dir, name), typeof data === "string" ? data : JSON.stringify(data, null, 2)) },
  }
}
