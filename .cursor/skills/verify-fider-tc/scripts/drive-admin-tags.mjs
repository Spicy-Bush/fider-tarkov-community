// Feature admin-tags: create a public and a private tag via /admin/tags as administrator, then prove from
// (1) the admin list after reload, (2) the anonymous visitor's tag list (public present, private absent), (3) the tags table.
// Limit: this proves tag visibility, NOT that filtering posts by tag works (no posts are created).
// Usage: control-fider drive drive-admin-tags.mjs [TagName]
import { launch, evidence, sql, URL_ } from "./lib.mjs"

const name = process.argv[2] || `verify-tag-${Date.now() % 100000}`
const slugOf = (n) => n.toLowerCase().replace(/[^a-z0-9]+/g, "-")
const slug = slugOf(name)
const privName = `${name} private`
const privSlug = slugOf(privName)
const ev = evidence("admin-tags")
const { browser, page } = await launch()

await page.goto(`${URL_}/admin/tags`)
await page.getByRole("heading", { name: "Public Tags" }).waitFor()
await ev.aria(page, "01-before")
async function createTag(tagName, visibility, shotName) {
  await page.getByRole("button", { name: "Add new" }).click()
  await page.getByRole("textbox", { name: "Name" }).fill(tagName)
  await page.locator(`#visibility-${visibility}`).check()
  if (shotName) await ev.shot(page, shotName)
  const save = page.waitForResponse((r) => new URL(r.url()).pathname === "/api/tags" && r.request().method() === "POST")
  await page.getByRole("button", { name: "Save" }).click()
  const resp = await save
  if (resp.status() !== 200) throw new Error(`save ${tagName} returned ${resp.status()}`)
  return { status: resp.status(), body: await resp.json().catch(() => null) }
}
const pubSave = await createTag(name, "public", "02-form-filled")
ev.write("03-save-response.json", pubSave)
const privSave = await createTag(privName, "private")
ev.write("03b-save-response-private.json", privSave)

await page.reload()
const pub = page.locator("main").getByText(name, { exact: true }).first()
await pub.waitFor()
await ev.aria(page, "04-after-reload")
await ev.shot(page, "04-after-reload")

const anon = await launch({ admin: false })
await anon.page.goto(`${URL_}/?tags=${slug}`)
await anon.page.waitForLoadState("networkidle")
const sd = JSON.parse(await anon.page.locator("#server-data").textContent())
const anonTags = sd.props?.tags
if (!Array.isArray(anonTags)) throw new Error("anon #server-data has no props.tags array")
const visibleToAnon = anonTags.some((t) => t.slug === slug && t.isPublic === true)
const privateLeaked = anonTags.some((t) => t.slug === privSlug)
ev.write("05b-anon-tags.json", anonTags)
await ev.shot(anon.page, "05-anon-home")
await anon.browser.close()

const row = sql(`select id,name,slug,color,is_public from tags where slug='${slug}'`)
const privRow = sql(`select id,name,slug,color,is_public from tags where slug='${privSlug}'`)
ev.write("06-db-row.txt", row + "\n" + privRow + "\n")
const result = { feature: "admin-tags", entry: "/admin/tags > Add new", name, slug, savedStatus: pubSave.status,
  listedAfterReload: true, publicTagInAnonTagList: visibleToAnon, privateTagInAnonTagList: privateLeaked, dbRow: row, privateDbRow: privRow,
  pass: !!row && row.endsWith("|t") && !!privRow && privRow.endsWith("|f") && visibleToAnon && !privateLeaked }
ev.write("result.json", result)
console.log(JSON.stringify(result))
await browser.close()
if (!result.pass) process.exit(1)
