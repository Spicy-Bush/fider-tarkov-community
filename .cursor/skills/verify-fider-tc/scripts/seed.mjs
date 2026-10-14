// Seed an empty DB through the real UI: /signup -> MailHog -> /signup/verify (site + admin),
// then save the admin browser session to $FIDER_RUN_DIR/admin-state.json.
import { launch, latestLink, URL_, ADMIN, RUN } from "./lib.mjs"
import path from "node:path"

const { browser, context, page } = await launch({ admin: false })
await page.goto(`${URL_}/signup`)
const signupForm = page.locator("#input-name")
const crashed = page.getByRole("heading", { name: /Shoot! Well, this is unexpected/ })
await signupForm.or(crashed).first().waitFor({ timeout: 15000 })
if (await signupForm.isVisible()) {
  await signupForm.fill("Verify Admin")
  await page.locator("#input-email").fill(ADMIN)
  await page.locator("#input-tenantName").fill("Tarkov Community (verify)")
  const legal = page.locator("#input-legalAgreement")
  if (await legal.count()) await legal.check()
  await page.getByRole("button", { name: "Confirm" }).click()
  await page.getByText("Click the link to complete the registration.").waitFor({ timeout: 15000 })
  console.log("seed: signup via /signup UI")
} else {
  // Known TC bug (see SKILL.md Gotchas): /signup crashes on an empty DB because
  // useAccountDraft reads fider.session.tenant.id while tenant is null. Fall back to the
  // exact JSON call the SignUp page makes, so the rest of the seed stays on real paths.
  console.log("seed: SIGNUP_UI_BROKEN (/signup error page) -> POST /api/tenants fallback")
  const r = await page.request.post(`${URL_}/api/tenants`, {
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    data: { name: "Verify Admin", email: ADMIN, tenantName: "Tarkov Community (verify)", legalAgreement: true },
  })
  if (!r.ok()) throw new Error(`POST /api/tenants ${r.status()} ${await r.text()}`)
}
await page.goto(await latestLink(ADMIN, "/signup/verify"))
await page.waitForLoadState("networkidle")
const sd = JSON.parse(await page.locator("#server-data").textContent())
if (!sd.user || sd.user.role !== "administrator") throw new Error("seed: not signed in as administrator after verify")
await context.storageState({ path: path.join(RUN, "admin-state.json") })
console.log(`seeded tenant + administrator ${ADMIN}`)
await browser.close()
