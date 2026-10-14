// Feature admin-tags: create a public tag via /admin/tags as administrator, then prove it from
// (1) the admin list after reload, (2) the anonymous home tag filter, (3) the tags table.
// Usage: control-fider drive drive-admin-tags.mjs [TagName]
import { launch, evidence, sql, URL_ } from "./lib.mjs"

const name = process.argv[2] || `verify-tag-${Date.now() % 100000}`
const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, "-")
const ev = evidence("admin-tags")
const { browser, page } = await launch()

await page.goto(`${URL_}/admin/tags`)
await page.getByRole("heading", { name: "Public Tags" }).waitFor()
await ev.aria(page, "01-before")
await page.getByRole("button", { name: "Add new" }).click()
await page.getByRole("textbox", { name: "Name" }).fill(name)
await page.locator("#visibility-public").check()
await ev.shot(page, "02-form-filled")
const save = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/tags" && r.request().method() === "POST")
await page.getByRole("button", { name: "Save" }).click()
const resp = await save
ev.write("03-save-response.json", { status: resp.status(), body: await resp.json().catch(() => null) })
if (resp.status() !== 200) throw new Error(`save returned ${resp.status()}`)

await page.reload()
const pub = page.locator("main").getByText(name, { exact: true }).first()
await pub.waitFor()
await ev.aria(page, "04-after-reload")
await ev.shot(page, "04-after-reload")

const anon = await launch({ admin: false })
await anon.page.goto(`${URL_}/?tags=${slug}`)
await anon.page.waitForLoadState("networkidle")
const sd = JSON.parse(await anon.page.locator("#server-data").textContent())
const visibleToAnon = JSON.stringify(sd.props || sd).includes(`"slug":"${slug}"`)
await ev.shot(anon.page, "05-anon-home")
await anon.browser.close()

const row = sql(`select id,name,slug,color,is_public from tags where slug='${slug}'`)
ev.write("06-db-row.txt", row + "\n")
const result = { feature: "admin-tags", entry: "/admin/tags > Add new", name, slug, savedStatus: resp.status(),
  listedAfterReload: true, inAnonServerData: visibleToAnon, dbRow: row, pass: !!row && row.endsWith("|t") && visibleToAnon }
ev.write("result.json", result)
console.log(JSON.stringify(result))
await browser.close()
if (!result.pass) process.exit(1)
