# tarkov.community verification map

This directory is the maintained source for verifying the user-facing behaviour of the tarkov.community / ru.tarkov.community site (Fider fork). Read this index before driving the app, then use the matching feature file as the recipe.

## Baseline preconditions

- Started with `control-fider up`, which serves a fresh DB at `http://localhost:3311` and reached `READY`.
- `control-fider doctor` prints `DOCTOR OK` with `BASE_URL=http://localhost:3311`.
- The seeded state is one tenant, `Tarkov Community (verify)`, with one administrator, `admin@verify.local` (`Verify Admin`). There are no posts, tags or campaigns.
- Never drive an instance that this run didn't start. Never drive tarkov.community, ru.tarkov.community or a `uat-*` host.

## Driving conventions

- Run every journey with `control-fider drive <script.mjs>` and Playwright headless from Shell.
- Prefer ARIA roles and accessible names (`getByRole`). Fall back to `#input-<field>` ids. Never use Tailwind classes.
- Each journey creates its own uniquely named fixtures through the UI first, for example a post via the home composer. Restoring them isn't needed because `down` discards the DB.
- Use the admin storageState by default, and `launch({ admin: false })` for visitor checks.

## Proof and skip reporting

- Capture the user action and the resulting state: a before/after ARIA snapshot, a screenshot showing the site header, and the save response.
- Mutation proof includes a second view: a reload, the anonymous visitor's view, or a `control-fider psql`/`sql()` row.
- Save artifacts under `/workspace/verify-artifacts/fider-tc/<RUN_ID>/<feature>/` together with `result.json` (`feature`, `entry`, `pass`).
- If a path is unreachable, report the attempted step and the unmet precondition. Don't report a skipped entry point as verified through a different path.

## Feature entry contract

Each feature file has an H1 and a one-paragraph description, then exactly four H2s in this order: `Sub-features`, `How to get to it (user POV)`, `Driving it with Playwright (control-fider)`, and `Gotchas`.

## Features

- [Post list, filters and search](./post-list.md) (**docs only, not driven**): the home composer, list, search box, tag filter, sort and the empty state.
- [Post page: votes and comments](./post-page.md) (**docs only, not driven**): show post, upvote/downvote, the discussion composer and sort.
- [Admin tags management](./admin-tags.md): create, edit and delete public/private tags, and visitor visibility. **Proven** for tag creation and anonymous tag-list visibility (`drive-admin-tags.mjs`); post filtering by tag is not proven.
- [Post status change](./post-status.md) (**docs only, not driven**): the staff Respond action, the status select and the status on the post and list.
- [Sponsorship / ads admin](./sponsorship.md) (**docs only, not driven**): ManageSponsorship tabs, campaigns, placements, the public slot and the AdSense fallback.
- EN vs RU locale isn't a separate feature, because production runs one tenant per locale. See the `locale-*` sub-features in [post-list](./post-list.md) and [sponsorship](./sponsorship.md).
