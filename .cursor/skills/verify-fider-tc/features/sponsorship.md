# Sponsorship / ads admin (ManageSponsorship)

`/admin/sponsorship` is the TC-only house-ad system. It has bookings (campaigns with creatives and a schedule), placements (fixed slots per page type and device, each with an empty-slot behaviour), exclusions and packages. Public pages request creatives with `POST /api/sponsorship/select`. When no booking fills a slot, the placement's `empty` mode decides what renders: nothing (`none`) or a Google **AdSense** fallback unit (`empty: "adsense"` plus `adsenseSlotId`, with the publisher from env `GOOGLE_ADSENSE`).

## Sub-features

- `sponsor-tabs` provides the **Bookings**, **Placements**, **Exclusions** and **Packages** tabs (a `tablist`).
- `sponsor-booking-create` uses **New campaign** to open the booking editor and save the booking, which then appears in the list. **Search bookings** filters the list.
- `sponsor-placement-edit` enables or disables a placement and sets the position, `every` (feed cadence) and the empty mode (none or adsense, plus the AdSense slot id).
- `sponsor-public-fill` renders an active booking in its slot on `/` or a post page, with tracked clicks via `/sponsorship/click`.
- `sponsor-adsense-fallback` makes an empty slot with `empty=adsense` render an AdSense unit (`data-ad-network="adsense"`, `data-ad-client`). With `none` it renders nothing.
- `sponsor-packages` covers package CRUD, shown on the public `/advertise` page.
- `locale-sponsor` uses the visitor locale (`en`/`ru`) for booking selection. The admin page itself is forced to English.

## How to get to it (user POV)

- Admin sidebar → **Site** → **Sponsorship** (`/admin/sponsorship`).
- The public **Advertise** header link (`/advertise`).

## Driving it with Playwright (control-fider)

Preconditions:

- Baseline admin storageState is in use. A fresh DB has no bookings ("No bookings found."), and all placements default to `empty=none`.
- Don't let the run fetch the real AdSense script. Use `page.route("**/pagead2.googlesyndication.com/**", r => r.abort())` and assert on the DOM attributes, not on rendered ads.

- **Tabs.** Go to `/admin/sponsorship`. `getByRole("tab", { name: "Bookings" })` is selected, and the tabs `Placements`, `Exclusions` and `Packages` exist.
- **Booking.** Click `getByRole("button", { name: "New campaign" })`, fill the editor (advertiser, creative and destination, schedule covering now, a placement), and save. The booking is listed and `getByRole("textbox", { name: "Search bookings" })` finds it.
- **Public fill.** As an anonymous visitor on `/` at a desktop viewport, the response to `POST /api/sponsorship/select` includes the booking, and the slot shows a "Sponsored" disclosure with the advertiser.
- **AdSense fallback.** With no booking for `home_desktop`, set the placement to `empty=adsense` with a dummy slot id in **Placements**, start fider with `GOOGLE_ADSENSE=ca-pub-0000000000000000` (edit `$RUN_DIR/app.env` and restart), then reload `/`. A node with `data-ad-network="adsense"` appears. Setting it back to `none` removes the node.
- **Proof.** Save `sponsorship/{admin.aria.txt,admin.png,select-response.json,home-slot.png}` and `select * from sponsor_placements`.

## Gotchas

- The living doc `12-domain-sponsorship.md` still describes the older `campaigns`/`slots` model (`/api/v1/sponsorship/*`). The `dev` code uses bookings and placements (`app/models/entity/sponsor_booking.go`, routes `/api/sponsorship/*`). Trust the code.
- Placements are split by device (`*_desktop` and `*_mobile`). Set the viewport to match the placement under test.
- `GOOGLE_ADSENSE` isn't set by `control-fider up`, so the AdSense fallback can't render until you add it. That's expected, not a failure.
- `/admin/sponsorship` forces the `en` locale (`SetLocale("en")`). RU checks belong on the public slots only.
