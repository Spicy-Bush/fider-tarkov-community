# Post status change

Staff change a post's status (open, planned, started, completed, declined, duplicate) with an optional response text. The status badge shows on the post and in the list, and status filters pick it up. TC content ops drive the same backend through the MCP.

## Sub-features

- `status-respond` uses the post actions dropdown → **Respond** to open the response modal with the `status` select and the response text.
- `status-set` saves a new status, which then shows on the post and in the list and is stored as `posts.status`.
- `status-duplicate` chooses Duplicate, then **Select Original Post**, and merges the votes into the original.
- `status-filter` shows the post under the matching status filter on `/`.

## How to get to it (user POV)

- On the post page, open the icon-only button next to the author and "Last activity" line (the actions `Dropdown`), then choose **Respond**.

## Driving it with Playwright (control-fider)

Preconditions:

- A post exists (see [post-list](./post-list.md)) and the admin storageState is in use.

- **Open actions.** On the post page, click `page.locator("main h1").locator("xpath=preceding::button[@aria-expanded][1]")`. Then click `getByText("Respond", { exact: true })`.
- **Choose status.** In the dialog, `#input-status` is a native select (combobox "Status": Open, Planned, Started, Completed, Duplicate, Declined). Use `selectOption({ label })` and optionally fill the "Reason" textbox. Click the dialog's `getByRole("button", { name: "Submit" })`. `POST /api/posts/<n>/status` returns 200.
- **Observe.** Submit reloads the page itself, so wait for the load instead of calling `page.reload()`. The post shows the new status badge. `select status from posts where number=<n>` changes from `0` (open).
- **List.** `/` with the status filter shows the post under the new status.
- **Proof.** Save `post-status/before.aria.txt`, `after.aria.txt`, `after.png`, and the DB row.

## Gotchas

- The actions trigger has no accessible name. That's an accessibility gap, so pin it relative to the post `h1` rather than by index. "Created …" is only a `data-tooltip` attribute, not text, so `getByText("Created")` finds nothing.
- Calling `page.reload()` right after Submit fails with `net::ERR_ABORTED`, because the dialog already reloads the page.
- Statuses are stored as ints (0 is open). Map them through the UI label and don't hard-code more than `0`.
- Duplicate needs a second post to exist first, so create two.
