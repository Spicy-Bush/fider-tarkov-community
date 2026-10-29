# Post list, filters and search

The home page (`/`) lists posts with a search box, tag and status filters, a date filter and a sort. It also hosts the "Enter your suggestion here..." composer, which creates the posts the other journeys need.

## Sub-features

- `list-empty` shows "No posts have been created yet." on a fresh DB.
- `list-compose` creates a post through the home composer, with a 30s send countdown and **Send Now**.
- `list-search` uses the query box to narrow the list (`?query=`) and handles a no-match state.
- `list-filter-tags` uses the tag filter (`?tags=<slug>`) to show only tagged posts.
- `list-filter-status` filters by status (open, planned, started, completed, declined, duplicate).
- `list-sort` changes the order with the sort control (trending, recent, most wanted and so on).
- `locale-list` shows UI strings in Russian when the tenant locale is `ru`.

## How to get to it (user POV)

- Click the site title link in the header (`getByRole("link", { name: "Tarkov Community (verify)" })`), or go to `/`.
- Use the filters and search above the list. The filter state is reflected in the query string.

## Driving it with Playwright (control-fider)

Preconditions:

- Baseline is up and you're using the admin storageState.
- Create two posts first: one with a unique word in the title and one tagged with a public tag from [admin-tags](./admin-tags.md).

- **Compose.** Fill `#input-title` (15 chars or more), then fill `getByRole("textbox", { name: "Describe your suggestion..." })` with 150 to 1000 chars. The description is required. Click `getByRole("button", { name: "Submit" })`. The text "Submitting in 30s..." appears with **Send Now** and **Cancel**. Click `getByRole("button", { name: "Send Now" })`. The URL becomes `/posts/<n>/<slug>`.
- **List.** As the admin, go to `/?view=trending` (any query param), or go to `/` as an anonymous visitor. Both posts are listed, and the empty-state paragraph is gone. The admin's bare `/` hides them (see Gotchas).
- **Search.** Fill `getByRole("textbox", { name: "Search" })` (`#input-query`, not a `searchbox` role) with the unique word and wait for the URL to gain `?query=`. Only the matching post remains. A nonsense query shows "No results matched your search, try something different." (not the fresh-DB "No posts have been created yet.").
- **Tag filter.** Go to `/?tags=<slug>` or use the tag filter. Only the tagged post remains.
- **Proof.** Save an ARIA snapshot and screenshot of the filtered list as `post-list/filtered.*`, plus `select number,title from posts`.
- **Locale.** On `/admin` General, set `Locale` to Russian and save, then reload `/`. The composer placeholder and headings are in Russian. Set it back afterwards.

## Gotchas

- The submit button stays disabled until the title has 15 or more characters and the description is empty or has 150 or more (tenant general settings). Don't shorten the journey to get around it.
- An empty description enables **Submit**, but the server rejects it (`POST /api/posts` 400) after the countdown, showing "Missing Translation: property.description is required." and a **Pending submissions** draft picker. Always fill a description of 150 chars or more.
- The 30s countdown is real. Either click **Send Now** or wait 30s or more. A fixed short sleep never posts.
- EN and RU are separate tenants and deployments in production. The local run proves locale rendering on one tenant and doesn't prove EN/RU data parity.
- Search is debounced. Wait for the list to change rather than using a fixed sleep.
- The bare `/` (empty query string) defaults to `notmyvotes=true` for a signed-in user (`app/handlers/post.go`, `Home`). Authors auto-vote their own posts, so the admin's bare `/` shows "No results matched your search" with **Filter 1** even though the posts exist. Add any query param (`/?view=trending`) or use the anonymous context. A saved filter cookie can do the same.
