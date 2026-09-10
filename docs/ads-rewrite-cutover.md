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
