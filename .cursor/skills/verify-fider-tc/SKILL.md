---
name: verify-fider-tc
description: "Drive a disposable local tarkov.community / ru.tarkov.community site (Spicy-Bush/fider-tarkov-community, Fider fork: Go + React/TS) through its web UI with headless Playwright from Shell: launch from an empty DB, seed an admin via MailHog magic link, run a feature journey, capture screenshots + ARIA + DB evidence, tear down. Use to prove UI behaviour (posts, votes, comments, tags, status, sponsorship, EN/RU) before a PR to dev. Never targets prod."
---

# Verify fider-tc (tarkov.community site)

Primary surface: the **web UI** (React SPA + Go SSR). Secondary surfaces, not covered here: the JSON API (`/api/...`, `/_api/...`) and the standalone MCP server (separate repo). Every step runs headless from Shell; no desktop browser is needed or used.

Everything lives in `.cursor/skills/verify-fider-tc/`:

| Path | What |
|------|------|
| `scripts/control-fider` | `build`, `up [RUN_ID]`, `doctor`, `drive <script.mjs> [args]`, `psql [args]`, `down` |
| `scripts/lib.mjs` | Playwright helpers: `launch({admin, locale})`, `latestLink(to, path)` (MailHog), `sql(q)`, `evidence(feature)` |
| `scripts/seed.mjs` | Run by `up`: creates the site + administrator through `/signup` and the MailHog verify link |
| `scripts/drive-admin-tags.mjs` | Proven journey for [admin-tags](features/admin-tags.md) |
| `features/` | The feature map. Read `features/README.md` before driving anything. |

Run every command from the repo root.

## Launch

The box has no Docker, so the stack runs natively. `dev-compose.yml` (pgdev/mailhog/minio on the external `bw-services` network) is the human dev setup, and verification does not use it.

One-time box prerequisites. Check each before installing it:

```bash
ls /usr/lib/postgresql/*/bin/initdb || sudo apt-get install -y postgresql make   # then: sudo service postgresql stop (don't leave the system cluster running)
test -x ~/.local/bin/MailHog || { mkdir -p ~/.local/bin && curl -sSL -o ~/.local/bin/MailHog https://github.com/mailhog/MailHog/releases/download/v1.0.1/MailHog_linux_amd64 && chmod +x ~/.local/bin/MailHog; }
```

Build. This runs `npm ci` if needed, `make build` (lingui + SSR + vite + `go build` → `./fider`), and installs Playwright's chromium-headless-shell for the repo's `@playwright/test` 1.57:

```bash
.cursor/skills/verify-fider-tc/scripts/control-fider build
```

`control-fider` sets `GOMODCACHE=/workspace/.vs-cache/gomod`, because the shared box cache (`~/go/pkg/mod`) once held a `rogchap.com/v8go` copy missing `deps/linux_x86_64/libv8.a`, which broke the link with `undefined reference to v8::...`. If the build fails that way again, run `GOMODCACHE=/workspace/.vs-cache/gomod go mod download rogchap.com/v8go`.

Start a run:

```bash
.cursor/skills/verify-fider-tc/scripts/control-fider up            # RUN_ID defaults to YYYYmmdd-HHMMSS
```

`up` does the following, in order. It refuses if any port is already bound or a run is already active.

1. `initdb` a fresh cluster in `/tmp/fider-verify-$RUN_ID/pg`, listening on `127.0.0.1:56543`, user `fider`/`fider_pw`, db `fider`.
2. MailHog on SMTP `127.0.0.1:11125` and API/UI `127.0.0.1:18125` (in-memory).
3. `./fider migrate`, then `./fider` (same order as the Dockerfile CMD), with `GO_ENV=development`, `HOST_MODE=single`, `BASE_URL=http://localhost:3311`, and a throwaway JWT secret. The env is written to `$RUN_DIR/app.env`.
4. It waits for `GET /_health` to return 200.
5. `seed.mjs` posts the signup and opens the `/signup/verify?k=` link from MailHog. It then asserts that `#server-data.user.role == "administrator"` and saves `admin-state.json` (Playwright storageState).

**Ready** means the final line `READY run=<id> url=http://localhost:3311 mailhog=http://127.0.0.1:18125 evidence=/workspace/verify-artifacts/fider-tc/<id>`. A healthy `/_health` alone is not ready, because the tenant and admin don't exist until the seed finishes. Override ports with `FIDER_VERIFY_PORT`, `FIDER_VERIFY_PG_PORT`, `FIDER_VERIFY_SMTP_PORT` and `FIDER_VERIFY_MH_PORT`.

Teardown is `control-fider down` (see Cleanup).

**Isolation:** the box is shared by several agents. Other agents run their own Postgres instances, for example one was seen on `:55432`. Only one verify run per checkout is allowed (`/tmp/fider-verify-current`). For a second concurrent run, use a separate worktree and set all four port variables. Never drive an instance this run didn't start.

## Doctor

Read-only. Run it first, and again whenever something looks off:

```bash
.cursor/skills/verify-fider-tc/scripts/control-fider doctor
```

It checks that the fider, postgres and mailhog PIDs it recorded are alive, that `:3311` is owned by *our* fider PID, that `/_health` returns 200, that `BASE_URL` is localhost (never prod), and that the admin session file exists. It prints `repo_head` and ends with `DOCTOR OK`. `server_build` shows `?` for local builds, because no `COMMITHASH` is passed. Compare `repo_head` with `git rev-parse --short HEAD` after rebuilding.

## Drive

The harness is Playwright (`@playwright/test` from the repo's `node_modules`), headless chromium, run as Node ESM scripts through `control-fider drive`. That command exports `FIDER_URL`, `FIDER_RUN_DIR`, `EVIDENCE_DIR`, `MAILHOG_API`, `ADMIN_EMAIL` and `PSQL`. `lib.mjs` throws on any non-`http://localhost:` URL.

```bash
.cursor/skills/verify-fider-tc/scripts/control-fider drive drive-admin-tags.mjs "Verify Tag"
.cursor/skills/verify-fider-tc/scripts/control-fider drive /path/to/your-journey.mjs   # write new journeys next to drive-admin-tags.mjs
.cursor/skills/verify-fider-tc/scripts/control-fider psql -At -c "select number,title,status from posts"
```

Writing a new journey: `import { launch, evidence, sql, latestLink, URL_ } from "./lib.mjs"`. Use `launch()` for the admin session and `launch({ admin: false })` for an anonymous visitor. To sign in as another user, open the sign-in modal, enter the email, then `page.goto(await latestLink(email, "/signin/verify"))`.

Stable handles in this repo (from real runs):

- Admin sidebar links: `getByRole("link", { name: "Tags" | "Sponsorship" | "Post Queue" | ... })`, with routes `/admin/tags`, `/admin/sponsorship` and so on.
- Bootstrap state: `JSON.parse(#server-data)` holds `.user`, `.tenant`, `.settings` and page `props`.
- Inputs use ids `#input-<field>` (`#input-title`, `#input-name`, `#input-color`). Radios use `#visibility-public` and `#visibility-private`.
- Prefer roles and names over CSS classes. Tailwind classes change constantly.

## Evidence

Evidence goes to `/workspace/verify-artifacts/fider-tc/<RUN_ID>/<feature>/`, outside the repo and outside the run dir, so it survives `down`. `down` also copies `fider.log`, `pg.log`, `migrate.log` and `mailhog.log` into `<RUN_ID>/`. Use `evidence(feature)`: `.shot(page, name)` saves a full-page PNG, `.aria(page, name)` saves an ARIA snapshot, and `.write(file, data)` saves JSON or text.

Proof standards:

- Drive the real user path, meaning clicks and typing in the UI. Don't call the API directly or set state internally. The only exception is the seed fallback described in Gotchas.
- Capture the action and the resulting state: a before ARIA snapshot, the filled form, the network response to the save, and the after state on reload.
- Verify side effects from a second view. Check the DB row with `sql(...)`, and check the anonymous view where the change should be public.
- Mock nothing. The only external systems are SMTP, which is the real MailHog, and Google AdSense. Don't load AdSense.
- Each journey writes `result.json` with `feature`, `entry`, the observed values and `pass`, and exits non-zero on failure.

## Cleanup

```bash
.cursor/skills/verify-fider-tc/scripts/control-fider down
```

It kills only the PIDs it recorded (fider, then MailHog), stops Postgres with `pg_ctl -D <run>/pg stop`, copies the logs into evidence, and removes `/tmp/fider-verify-<RUN_ID>` and the `/tmp/fider-verify-current` link. It never kills by process name and never touches the evidence. Run it after every failed attempt too, because a failed `up` leaves processes running on purpose so you can inspect them. Then check that the ports are free: `ss -ltn | grep -E ':(3311|56543|11125|18125)\b' || echo free`. The build outputs (`./fider`, `dist/`, `ssr.js`, `node_modules`) are gitignored and stay in place.

## Gotchas

- **`/signup` crashes on an empty DB (TC bug, `dev` @ 5e08abbd).** `public/hooks/useAccountDraft.ts` builds ``${fider.session.tenant.id}:...`` while the tenant is null, so the page shows "Shoot! Well, this is unexpected… Cannot read properties of null (reading 'id')". `seed.mjs` detects this, logs `seed: SIGNUP_UI_BROKEN`, and falls back to the exact `POST /api/tenants` JSON call the signup page makes. Activation still goes through the real MailHog `/signup/verify` link. Once the bug is fixed, the seed uses the UI path automatically (`seed: signup via /signup UI`).
- Mutations must send JSON `Accept`/`Content-Type` (CSRF/ajax gate). The UI does this. Hand-written `page.request` calls must too.
- Fresh DBs have no tags, posts, sponsorship packages or campaigns. Each journey creates its own fixtures with unique names, and `down` discards the whole DB.
- The admin is created as `administrator` with the email `admin@verify.local`. Helper and moderator journeys need extra users invited through `/admin/members`.
- The red `DEV` badge in the header means `GO_ENV=development`. Prod doesn't show it.
