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

- **Compose.** Fill `#input-title` (15 chars or more), then fill `getByRole("textbox", { name: "Describe your suggestion..." })` (empty or 150 chars or more). Click `getByRole("button", { name: "Submit" })`. The text "Submitting in 30s..." appears with **Send Now** and **Cancel**. Click `getByRole("button", { name: "Send Now" })`. The URL becomes `/posts/<n>/<slug>`.
- **List.** Go to `/`. Both posts are listed, and the empty-state paragraph is gone.
- **Search.** Type the unique word into the list search box. Only the matching post remains. A nonsense query shows the empty state.
- **Tag filter.** Go to `/?tags=<slug>` or use the tag filter. Only the tagged post remains.
- **Proof.** Save an ARIA snapshot and screenshot of the filtered list as `post-list/filtered.*`, plus `select number,title from posts`.
- **Locale.** On `/admin` General, set `Locale` to Russian and save, then reload `/`. The composer placeholder and headings are in Russian. Set it back afterwards.

## Gotchas

- The submit button stays disabled until the title has 15 or more characters and the description is empty or has 150 or more (tenant general settings). Don't shorten the journey to get around it.
- The 30s countdown is real. Either click **Send Now** or wait 30s or more. A fixed short sleep never posts.
- EN and RU are separate tenants and deployments in production. The local run proves locale rendering on one tenant and doesn't prove EN/RU data parity.
- Search is debounced. Wait for the list to change rather than using a fixed sleep.
