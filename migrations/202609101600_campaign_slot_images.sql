-- Per-slot creative images: map slot_id -> image URL (JSONB).
-- Keeps creative_image_url as legacy fallback when a slot key is missing.

alter table sponsorship_campaigns
  add column if not exists creative_image_urls jsonb not null default '{}'::jsonb;

-- Backfill: copy legacy creative_image_url into the map for each selected slot.
update sponsorship_campaigns c
set creative_image_urls = (
  select coalesce(jsonb_object_agg(slot_id, c.creative_image_url), '{}'::jsonb)
  from (
    select trim(both from unnest(string_to_array(c.slots, ','))) as slot_id
  ) s
  where slot_id <> ''
    and c.creative_image_url is not null
    and c.creative_image_url <> ''
)
where c.creative_image_url is not null
  and c.creative_image_url <> ''
  and (c.creative_image_urls = '{}'::jsonb or c.creative_image_urls is null);
