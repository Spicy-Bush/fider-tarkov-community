-- Public advertiser / company name for EU-style disclosure (shown outside creative).
-- Campaign name remains internal (billing/reference).

alter table sponsorship_campaigns
  add column if not exists advertiser text not null default '';

-- Backfill from internal name so existing UAT campaigns keep a public label.
update sponsorship_campaigns
set advertiser = name
where advertiser = '' or advertiser is null;
