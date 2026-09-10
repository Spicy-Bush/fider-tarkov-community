-- P0b: optional AdSense empty-placement fallback metadata on catalog rows.
-- Slot IDs live on placements (not campaigns / creatives / assignments / select).
-- empty_policy: collapse (default, render nothing) | reserve (keep placement frame space).
-- adsense_slot_id stays empty until set via PUT /api/v1/ads/placements/:id (no live publisher
-- ids hardcoded). Format defaults below are AdSense-friendly hints only.

ALTER TABLE ad_placements
  ADD COLUMN IF NOT EXISTS adsense_slot_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS adsense_format text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS empty_policy text NOT NULL DEFAULT 'collapse';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'chk_ad_placements_empty_policy'
      AND conrelid = 'ad_placements'::regclass
  ) THEN
    ALTER TABLE ad_placements
      ADD CONSTRAINT chk_ad_placements_empty_policy
      CHECK (empty_policy IN ('collapse', 'reserve'));
  END IF;
END $$;

-- Seed empty_policy + format for known placements; leave adsense_slot_id empty for admin/API.
UPDATE ad_placements SET empty_policy = 'collapse' WHERE empty_policy IS NULL OR empty_policy = '';
UPDATE ad_placements SET adsense_format = 'fluid' WHERE id = 'feed_native' AND adsense_format = '';
UPDATE ad_placements SET adsense_format = 'rectangle' WHERE id = 'sidebar_top' AND adsense_format = '';
UPDATE ad_placements SET adsense_format = 'horizontal' WHERE id = 'post_below_title' AND adsense_format = '';
UPDATE ad_placements SET adsense_format = 'horizontal' WHERE id = 'pages_header' AND adsense_format = '';
