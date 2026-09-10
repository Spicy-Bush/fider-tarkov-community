-- Allow one campaign/sponsor to target multiple placements.
-- Safe if 202609101200 already created `slots` instead of `slot_id`.

do $$
begin
  if exists (
    select 1 from information_schema.columns
    where table_name = 'sponsorship_campaigns' and column_name = 'slot_id'
  ) and not exists (
    select 1 from information_schema.columns
    where table_name = 'sponsorship_campaigns' and column_name = 'slots'
  ) then
    alter table sponsorship_campaigns add column slots text not null default '';
    update sponsorship_campaigns set slots = slot_id where slots = '' and slot_id is not null and slot_id <> '';
    alter table sponsorship_campaigns drop constraint if exists chk_sponsorship_campaigns_slot;
    alter table sponsorship_campaigns drop column slot_id;
  end if;
end $$;
