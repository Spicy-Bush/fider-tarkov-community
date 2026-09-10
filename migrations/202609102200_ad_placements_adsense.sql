-- P0b: optional AdSense empty-placement fallback metadata on catalog rows.
-- Slot IDs live on placements (not campaigns / creatives / assignments / select).
-- empty_policy: collapse (default, render nothing) | reserve (keep placement frame space).

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
