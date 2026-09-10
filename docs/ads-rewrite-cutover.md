# Ads rewrite cutover (expand -> contract)

## Deploy model (single cutover)

Portainer deploys the whole `dev` image **atomically**. On boot the binary runs migrations in order:

1. `202609102000_ads_rewrite_expand` - placements, versions, assignments, OCC, backfill
2. `202609102050_ads_ownership_softdelete` - composite ownership FK, soft-delete, RESTRICT on creatives
3. `202609102100_ads_rewrite_contract` - **DROP** fat campaign columns (`slots`, `creative_*`, `click_url`)

There is **no rolling multi-version** window: old pods must not keep serving against a contracted schema. Treat this as recreate / single-cutover only.

## Rollback

Redeploy the **previous image**. Note: contract migration drops columns - that step is **one-way**. Rolling back the app without restoring those columns from backup will break any binary that still reads fat fields. UAT must verify select + admin graph after deploy before promoting further.

## Soft-delete / creatives (Zaddish #8)

Campaign delete is a **soft-delete** (`deleted_at`). `creative_versions` FK to campaigns is `ON DELETE RESTRICT` so a hard DELETE cannot wipe published assets. Selection skips soft-deleted campaigns. Hard GC of orphaned versions is out of v1 scope.

## Assignment ownership

`campaign_assignments` references `creative_versions (tenant_id, id, campaign_id)` so a version cannot be bound under a different campaign. Graph save also validates ownership in-app before insert.

## Admin writes

- `PUT /api/v1/sponsorship/campaigns/:id/graph` - campaign fields + full assignment set in **one** DB transaction with OCC (`config_version`).
- `POST .../versions` - append immutable creative; requires `configVersion` and bumps OCC.
- Standalone assignment PUT/DELETE routes were removed; FE stages assignments and persists via graph save.

## Package slots CSV

`sponsorship_packages.slots` remains a free-text CSV of placement ids for **/advertise** marketing copy only. Live selection reads `campaign_assignments` + `ad_placements` — it never parses package slots.

## AdSense empty-placement fallback (P0b)

Migration `202609102200_ad_placements_adsense` adds `adsense_slot_id`, `adsense_format`, `empty_policy` on `ad_placements` and seeds format hints for known placements (`feed_native` fluid, `sidebar_top` rectangle, `post_below_title` / `pages_header` horizontal). Slot IDs stay empty (no live publisher/slot ids in repo).

### Enable without raw SQL

1. Set `GOOGLE_ADSENSE` to your `ca-pub-...` client (env / boot inject). Do not put publisher id in the DB.
2. Create AdSense ad units in Google, then set each placement slot via either:
   - Admin UI: `/admin/sponsorship` → **Placements** tab → Save
   - API (collab/admin): `PUT /api/v1/ads/placements/:id` with JSON:
     `{ "adsenseSlotId": "<unit-slot-id>", "adsenseFormat": "auto", "emptyPolicy": "collapse" }`
3. Public clients read slim config from `GET /api/v1/ads/placement-config` when house `ads/select` returns null.
4. AdSense chrome does **not** render TC "Sponsored" (Google labels its own units). House fill still requires a non-blank advertiser (`Sponsored - {name}`).

### empty_policy

- `collapse` (default): render nothing when no house fill and no AdSense slot/client
- `reserve`: keep the placement frame space empty
