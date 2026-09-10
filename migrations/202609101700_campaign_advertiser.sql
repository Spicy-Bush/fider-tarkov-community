-- Public advertiser / company name for EU-style disclosure (shown outside creative).
-- Campaign name remains internal (billing/reference).

alter table sponsorship_campaigns
  add column if not exists advertiser text not null default '';

-- Backfill from internal name so existing UAT campaigns keep a public label.
update sponsorship_campaigns
set advertiser = name
where btrim(advertiser) = '' or advertiser is null;

-- Reject empty advertiser at DB layer (after backfill).
alter table sponsorship_campaigns
  alter column advertiser drop default;

alter table sponsorship_campaigns
  drop constraint if exists sponsorship_campaigns_advertiser_nonempty;

alter table sponsorship_campaigns
  add constraint sponsorship_campaigns_advertiser_nonempty
  check (btrim(advertiser) <> '');
