-- PHASE 1 expand: ad_placements catalog, creative_versions, campaign_assignments,
-- config_version OCC, composite package FK, backfill from fat campaign columns.
-- DO NOT DROP slots / creative_* / click_url yet (contract is a later phase).

-- ---------------------------------------------------------------------------
-- 1. Global placement catalog
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ad_placements (
  id          text PRIMARY KEY,
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  kind        text NOT NULL,
  max_width   int,
  max_height  int,
  sort        int NOT NULL DEFAULT 0,
  enabled     boolean NOT NULL DEFAULT true
);

INSERT INTO ad_placements (id, name, description, kind, max_width, max_height, sort, enabled)
VALUES
  ('feed_native',      'Feed (native)',     'Native card in the home feed',              'native', 1200, 675, 10, true),
  ('sidebar_top',      'Sidebar',           'Top of home sidebar',                       'frame',  600, 500, 20, true),
  ('post_below_title', 'Below post title',  'Banner under post title on ShowPost',       'frame', 1200, 400, 30, true),
  ('pages_header',     'Pages header',      'Banner at top of content pages',            'frame', 1200, 280, 40, true)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. UNIQUE (tenant_id, id) for composite FKs
-- ---------------------------------------------------------------------------
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'uq_sponsorship_campaigns_tenant_id'
      AND conrelid = 'sponsorship_campaigns'::regclass
  ) THEN
    ALTER TABLE sponsorship_campaigns
      ADD CONSTRAINT uq_sponsorship_campaigns_tenant_id UNIQUE (tenant_id, id);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'uq_sponsorship_packages_tenant_id'
      AND conrelid = 'sponsorship_packages'::regclass
  ) THEN
    ALTER TABLE sponsorship_packages
      ADD CONSTRAINT uq_sponsorship_packages_tenant_id UNIQUE (tenant_id, id);
  END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 3. config_version OCC column
-- ---------------------------------------------------------------------------
ALTER TABLE sponsorship_campaigns
  ADD COLUMN IF NOT EXISTS config_version int NOT NULL DEFAULT 1;

-- ---------------------------------------------------------------------------
-- 4. creative_versions (immutable payloads)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS creative_versions (
  id          serial,
  tenant_id   int NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  campaign_id int NOT NULL,
  version_no  int NOT NULL,
  image_url   text NOT NULL DEFAULT '',
  html        text NOT NULL DEFAULT '',
  click_url   text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, campaign_id, version_no),
  UNIQUE (tenant_id, id, campaign_id),
  FOREIGN KEY (tenant_id, campaign_id)
    REFERENCES sponsorship_campaigns (tenant_id, id) ON DELETE RESTRICT,
  CONSTRAINT chk_creative_versions_click_url CHECK (click_url <> ''),
  CONSTRAINT chk_creative_versions_surface CHECK (image_url <> '' OR html <> '')
);

CREATE INDEX IF NOT EXISTS idx_creative_versions_campaign
  ON creative_versions (tenant_id, campaign_id);

-- ---------------------------------------------------------------------------
-- 5. campaign_assignments (placement → creative_version under a campaign)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS campaign_assignments (
  id                  serial,
  tenant_id           int NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  campaign_id         int NOT NULL,
  placement_id        text NOT NULL REFERENCES ad_placements(id),
  creative_version_id int NOT NULL,
  PRIMARY KEY (tenant_id, id),
  UNIQUE (tenant_id, campaign_id, placement_id),
  FOREIGN KEY (tenant_id, campaign_id)
    REFERENCES sponsorship_campaigns (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, creative_version_id, campaign_id)
    REFERENCES creative_versions (tenant_id, id, campaign_id)
);

CREATE INDEX IF NOT EXISTS idx_campaign_assignments_placement
  ON campaign_assignments (tenant_id, placement_id);

CREATE INDEX IF NOT EXISTS idx_campaign_assignments_campaign
  ON campaign_assignments (tenant_id, campaign_id);

-- ---------------------------------------------------------------------------
-- 6. Backfill creative_versions (A2)
--    Prefer distinct non-empty URLs from creative_image_urls jsonb;
--    else one version from legacy creative_image_url + creative_html + click_url.
-- ---------------------------------------------------------------------------

-- 6a. From creative_image_urls map (one version per distinct non-empty URL)
INSERT INTO creative_versions (tenant_id, campaign_id, version_no, image_url, html, click_url)
SELECT
  src.tenant_id,
  src.campaign_id,
  src.version_no,
  src.image_url,
  src.html,
  src.click_url
FROM (
  SELECT
    c.tenant_id,
    c.id AS campaign_id,
    e.value AS image_url,
    COALESCE(c.creative_html, '') AS html,
    c.click_url,
    row_number() OVER (
      PARTITION BY c.tenant_id, c.id
      ORDER BY e.value
    ) AS version_no
  FROM sponsorship_campaigns c
  CROSS JOIN LATERAL (
    SELECT DISTINCT btrim(j.value) AS value
    FROM jsonb_each_text(COALESCE(c.creative_image_urls, '{}'::jsonb)) j
    WHERE btrim(j.value) <> ''
  ) e
  WHERE btrim(c.click_url) <> ''
    AND c.creative_image_urls IS NOT NULL
    AND c.creative_image_urls <> '{}'::jsonb
    AND NOT EXISTS (
      SELECT 1 FROM creative_versions cv
      WHERE cv.tenant_id = c.tenant_id AND cv.campaign_id = c.id
    )
) src
WHERE src.image_url <> '' OR src.html <> '';

-- 6b. Legacy single creative for campaigns still without versions
INSERT INTO creative_versions (tenant_id, campaign_id, version_no, image_url, html, click_url)
SELECT
  c.tenant_id,
  c.id,
  1,
  COALESCE(c.creative_image_url, ''),
  COALESCE(c.creative_html, ''),
  c.click_url
FROM sponsorship_campaigns c
WHERE btrim(c.click_url) <> ''
  AND (
    btrim(COALESCE(c.creative_image_url, '')) <> ''
    OR btrim(COALESCE(c.creative_html, '')) <> ''
  )
  AND NOT EXISTS (
    SELECT 1 FROM creative_versions cv
    WHERE cv.tenant_id = c.tenant_id AND cv.campaign_id = c.id
  );

-- ---------------------------------------------------------------------------
-- 7. Backfill campaign_assignments from slots CSV
--    Prefer version whose image_url matches map[slot]; else lowest version_no.
-- ---------------------------------------------------------------------------
INSERT INTO campaign_assignments (tenant_id, campaign_id, placement_id, creative_version_id)
SELECT
  c.tenant_id,
  c.id,
  s.placement_id,
  COALESCE(
    (
      SELECT cv.id
      FROM creative_versions cv
      WHERE cv.tenant_id = c.tenant_id
        AND cv.campaign_id = c.id
        AND btrim(COALESCE(c.creative_image_urls ->> s.placement_id, '')) <> ''
        AND cv.image_url = btrim(c.creative_image_urls ->> s.placement_id)
      ORDER BY cv.version_no ASC
      LIMIT 1
    ),
    (
      SELECT cv.id
      FROM creative_versions cv
      WHERE cv.tenant_id = c.tenant_id AND cv.campaign_id = c.id
      ORDER BY cv.version_no ASC
      LIMIT 1
    )
  ) AS creative_version_id
FROM sponsorship_campaigns c
CROSS JOIN LATERAL (
  SELECT DISTINCT btrim(x) AS placement_id
  FROM unnest(string_to_array(COALESCE(c.slots, ''), ',')) AS x
  WHERE btrim(x) <> ''
) s
WHERE EXISTS (SELECT 1 FROM ad_placements p WHERE p.id = s.placement_id)
  AND EXISTS (
    SELECT 1 FROM creative_versions cv
    WHERE cv.tenant_id = c.tenant_id AND cv.campaign_id = c.id
  )
  AND NOT EXISTS (
    SELECT 1 FROM campaign_assignments a
    WHERE a.tenant_id = c.tenant_id
      AND a.campaign_id = c.id
      AND a.placement_id = s.placement_id
  );

-- ---------------------------------------------------------------------------
-- 8. Replace package_id FK with composite (tenant_id, package_id)
-- ---------------------------------------------------------------------------
DO $$
DECLARE
  fk_name text;
BEGIN
  FOR fk_name IN
    SELECT con.conname
    FROM pg_constraint con
    JOIN pg_attribute att
      ON att.attrelid = con.conrelid AND att.attnum = ANY (con.conkey)
    WHERE con.conrelid = 'sponsorship_campaigns'::regclass
      AND con.contype = 'f'
      AND att.attname = 'package_id'
  LOOP
    EXECUTE format('ALTER TABLE sponsorship_campaigns DROP CONSTRAINT %I', fk_name);
  END LOOP;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'sponsorship_campaigns_tenant_package_fk'
      AND conrelid = 'sponsorship_campaigns'::regclass
  ) THEN
    ALTER TABLE sponsorship_campaigns
      ADD CONSTRAINT sponsorship_campaigns_tenant_package_fk
      FOREIGN KEY (tenant_id, package_id)
      REFERENCES sponsorship_packages (tenant_id, id)
      ON DELETE SET NULL;
  END IF;
END $$;
