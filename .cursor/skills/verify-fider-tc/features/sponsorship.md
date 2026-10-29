# Sponsorship / ads admin (ManageSponsorship)

`/admin/sponsorship` is the TC-only house-ad system. It has bookings (campaigns with creatives and a schedule), placements (fixed slots per page type and device, each with an empty-slot behaviour), exclusions and packages. Public pages request creatives with `POST /api/sponsorship/select`. When no booking fills a slot, the placement's `empty` mode decides what renders: nothing (`none`), a Ko-fi support slot (`kofi`), or a Google **AdSense** fallback unit (`empty: "adsense"` plus `adsenseSlotId`, with the publisher from env `GOOGLE_ADSENSE`).

## Sub-features

- `sponsor-tabs` provides the **Bookings**, **Placements**, **Exclusions** and **Packages** tabs (a `tablist`).
- `sponsor-booking-create` uses **New campaign** to open the booking editor and save the booking, which then appears in the list. **Search bookings** filters the list.
- `sponsor-placement-edit` enables or disables a placement and sets the position, `every` (feed cadence) and the empty mode (none, kofi or adsense, plus the AdSense slot id).
- `sponsor-public-fill` renders an active booking in its slot on `/` or a post page, with tracked clicks via `/sponsorship/click`.
- `sponsor-adsense-fallback` makes an empty slot with `empty=adsense` render an AdSense unit (`data-ad-network="adsense"`, `data-ad-client`). With `none` it renders nothing.
- `sponsor-packages` covers package CRUD, shown on the public `/advertise` page.
- `locale-sponsor` uses the visitor locale (`en`/`ru`) for booking selection. The admin page itself is forced to English.

## How to get to it (user POV)

- Admin sidebar → **Site** → **Sponsorship** (`/admin/sponsorship`).
- The public **Advertise** header link (`/advertise`).

## Driving it with Playwright (control-fider)

Preconditions:

- Baseline admin storageState is in use. A fresh DB has no bookings ("No bookings found.") and no rows in `sponsor_placement_settings`, so every placement is **disabled** with `empty=none`. A disabled placement never renders, so enable the one under test first.
- Don't let the run fetch the real AdSense script. Use `page.route("**/pagead2.googlesyndication.com/**", r => r.abort())` and assert on the DOM attributes, not on rendered ads.

- **Tabs.** Go to `/admin/sponsorship`. `getByRole("tab", { name: "Bookings" })` is selected, and the tabs `Placements`, `Exclusions` and `Packages` exist.
- **Placement.** Click `getByRole("tab", { name: "Placements" })`. Each placement is an `article` with its name as heading, a `checkbox` "Enabled", a `combobox` "Unbooked inventory" (Leave empty, Show Ko-fi, Show Google AdSense) and its own **Save** (`POST /api/sponsorship/placements`). For `home_desktop`, check "Enabled" on "Home sidebar" and save.
- **Booking.** Click `getByRole("button", { name: "New campaign" })`. Fill textboxes "Booking name", "Advertiser" and "Destination URL". "Headline" sits in a collapsed "Text and styling" section, so click `getByText("Text and styling")` first. Set combobox "Review status" to Approved and "Booking status" to Booked (both are required for delivery), check the placement checkbox (e.g. "Home sidebar") and set its "<placement> share" to 100, then click `getByRole("button", { name: "Save", exact: true })`. The booking is listed and `getByRole("textbox", { name: "Search bookings" })` finds it.
- **Public fill.** As an anonymous visitor on `/` at a desktop viewport, the response to `POST /api/sponsorship/select` has `home_desktop.kind == "sponsor"`, and the slot is `getByRole("complementary", { name: "Advertisement from <advertiser>" })` with the text "Advertisement" (not "Sponsored") and a link named after the headline to `/sponsorship/click?token=...`.
- **AdSense fallback.** Restart fider with the publisher id: `D=$RUN_DIR; kill $(cat $D/fider.pid); echo GOOGLE_ADSENSE=ca-pub-0000000000000000 >> $D/app.env; ( cd <repo> && exec env $(cat "$D/app.env" | xargs) ./fider ) >>"$D/fider.log" 2>&1 & echo $! > $D/fider.pid`, then run `control-fider doctor`. Pick an unbooked placement (e.g. "Community strip on desktop", `strip_desktop`), check "Enabled", choose "Show Google AdSense", fill "AdSense ad unit ID" with a dummy number and save, then reload `/` anonymously. A node with `data-ad-network="adsense"` appears. Setting it back to "Leave empty" (`none`) removes the node.
- **Proof.** Save `sponsorship/{admin.aria.txt,admin.png,select-response.json,home-slot.png}` and `select * from sponsor_placement_settings` (there is no `sponsor_placements` table; campaign name and advertiser live in `sponsor_campaigns.data` jsonb).

## Gotchas

- The living doc `12-domain-sponsorship.md` still describes the older `campaigns`/`slots` model (`/api/v1/sponsorship/*`). The `dev` code uses bookings and placements (`app/models/entity/sponsor_booking.go`, routes `/api/sponsorship/*`). Trust the code.
- Placements are split by device (`*_desktop` and `*_mobile`). Set the viewport to match the placement under test.
- `GOOGLE_ADSENSE` isn't set by `control-fider up`, so the AdSense fallback can't render until you add it. That's expected, not a failure.
- `/admin/sponsorship` forces the `en` locale (`SetLocale("en")`). RU checks belong on the public slots only.
- A booking's placement share defaults to 50%, so the other half of selections return `kind: "none"` for that slot. Set the share to 100 for a deterministic public-fill check.
