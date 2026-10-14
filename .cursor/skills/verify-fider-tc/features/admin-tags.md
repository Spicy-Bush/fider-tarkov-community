# Admin tags management

Staff (Collaborator or Administrator) create, edit and delete tags on `/admin/tags`. Public tags show to every visitor and can filter the post list. Private tags show to members only. Content ops and the MCP `sync_tags` tool depend on this taxonomy.

## Sub-features

- `tags-create-public` adds a public tag, which appears under **Public Tags** and is visible to an anonymous visitor.
- `tags-create-private` adds a private tag, which appears under **Private Tags** and is hidden from an anonymous visitor.
- `tags-edit` renames a tag or changes its colour or visibility with **Edit**.
- `tags-delete` removes a tag with **Delete** and a confirmation.
- `tags-assign` uses **Edit tags** on a post page to assign or unassign a tag (covered from the post side in [post-page](./post-page.md)).

## How to get to it (user POV)

- Admin sidebar → **Site** → **Tags** (`/admin/tags`).
- Type `/admin/tags` directly while signed in as staff.

## Driving it with Playwright (control-fider)

Preconditions:

- Baseline run is up, `doctor` is OK, and you're using the admin storageState.
- No tag exists with the chosen name. Use a unique name per run.

- **Proven journey.** Run `control-fider drive drive-admin-tags.mjs "Verify Tag"`. It writes `admin-tags/{01-before.aria.txt,02-form-filled.png,03-save-response.json,04-after-reload.png,04-after-reload.aria.txt,05-anon-home.png,06-db-row.txt,result.json}` and exits 0 with `"pass":true`.
- **Open form.** Click `getByRole("button", { name: "Add new" })`. The `Name` textbox, the `Color` textbox (prefilled with a random hex) and the `Public`/`Private` radios appear, with `Private` checked by default.
- **Fill.** Fill `getByRole("textbox", { name: "Name" })` and `.check()` `#visibility-public`. The preview chip updates.
- **Save.** Click `getByRole("button", { name: "Save" })`. `POST /api/tags` returns 200 with `{ id, name, slug, color, isPublic }`.
- **Persisted.** `page.reload()`. The name is listed under **Public Tags** with **Edit** and **Delete** buttons.
- **Visitor sees it.** In an anonymous context, `#server-data` on `/` contains `"slug":"<slug>"`.
- **DB.** `select id,name,slug,color,is_public from tags where slug='<slug>'` ends in `|t`.
- **Private variant.** Leave `Private` checked. The anonymous `#server-data` must **not** contain the slug.

## Gotchas

- The slug is derived from the name (lowercase, spaces become `-`). Assert on the slug and not only the display name.
- The save endpoint is `/api/tags` and not `/api/v1/tags`. A `waitForResponse` on the wrong path hangs for 30s.
- Saving the same name twice on one run fails validation. Use unique names.
- Helper and Moderator can't open `/admin/tags` (Collaborator+ only), so the role-gate check needs an invited non-staff user.
