-- Phase 3 contract: drop fat creative/slots columns from sponsorship_campaigns.
-- Safe only after Phase 1 backfill (versions/assignments) and Phase 2 app cutover
-- (selection + admin read the assignment graph; FE uses POST /api/v1/ads/select).

ALTER TABLE sponsorship_campaigns
  DROP COLUMN IF EXISTS slots,
  DROP COLUMN IF EXISTS creative_image_url,
  DROP COLUMN IF EXISTS creative_image_urls,
  DROP COLUMN IF EXISTS creative_html,
  DROP COLUMN IF EXISTS click_url;
