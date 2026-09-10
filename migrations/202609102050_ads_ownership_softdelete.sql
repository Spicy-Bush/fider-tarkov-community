-- Assignment ownership (composite FK), soft-delete campaigns, stop CASCADE wiping creatives.
-- Choice (Zaddish #8): soft-delete campaigns (deleted_at); creative_versions FK is RESTRICT
-- so hard DELETE cannot destroy published assets. Assignments stay; selection skips deleted.
-- GC of orphaned versions is out of v1 scope.

-- ---------------------------------------------------------------------------
-- 1. Soft-delete column on campaigns
-- ---------------------------------------------------------------------------
ALTER TABLE sponsorship_campaigns
  ADD COLUMN IF NOT EXISTS deleted_at timestamptz NULL;

CREATE INDEX IF NOT EXISTS idx_sponsorship_campaigns_not_deleted
  ON sponsorship_campaigns (tenant_id)
  WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- 2. creative_versions: UNIQUE (tenant_id, id, campaign_id) for composite FK
-- ---------------------------------------------------------------------------
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'uq_creative_versions_tenant_id_campaign'
      AND conrelid = 'creative_versions'::regclass
  ) THEN
    ALTER TABLE creative_versions
      ADD CONSTRAINT uq_creative_versions_tenant_id_campaign
      UNIQUE (tenant_id, id, campaign_id);
  END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 3. Replace CASCADE campaign FK on creative_versions with RESTRICT
-- ---------------------------------------------------------------------------
DO $$
DECLARE
  fk_name text;
BEGIN
  FOR fk_name IN
    SELECT con.conname
    FROM pg_constraint con
    JOIN pg_class rel ON rel.oid = con.conrelid
    WHERE rel.relname = 'creative_versions'
      AND con.contype = 'f'
      AND pg_get_constraintdef(con.oid) ILIKE '%sponsorship_campaigns%'
  LOOP
    EXECUTE format('ALTER TABLE creative_versions DROP CONSTRAINT %I', fk_name);
  END LOOP;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'creative_versions_campaign_fk'
      AND conrelid = 'creative_versions'::regclass
  ) THEN
    ALTER TABLE creative_versions
      ADD CONSTRAINT creative_versions_campaign_fk
      FOREIGN KEY (tenant_id, campaign_id)
      REFERENCES sponsorship_campaigns (tenant_id, id)
      ON DELETE RESTRICT;
  END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 4. campaign_assignments: composite FK enforces version belongs to same campaign
-- ---------------------------------------------------------------------------
DO $$
DECLARE
  fk_name text;
BEGIN
  -- Drop FKs that reference creative_versions (old tenant_id,id only)
  FOR fk_name IN
    SELECT con.conname
    FROM pg_constraint con
    JOIN pg_class rel ON rel.oid = con.conrelid
    WHERE rel.relname = 'campaign_assignments'
      AND con.contype = 'f'
      AND pg_get_constraintdef(con.oid) ILIKE '%creative_versions%'
  LOOP
    EXECUTE format('ALTER TABLE campaign_assignments DROP CONSTRAINT %I', fk_name);
  END LOOP;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'campaign_assignments_creative_version_campaign_fk'
      AND conrelid = 'campaign_assignments'::regclass
  ) THEN
    ALTER TABLE campaign_assignments
      ADD CONSTRAINT campaign_assignments_creative_version_campaign_fk
      FOREIGN KEY (tenant_id, creative_version_id, campaign_id)
      REFERENCES creative_versions (tenant_id, id, campaign_id);
  END IF;
END $$;
