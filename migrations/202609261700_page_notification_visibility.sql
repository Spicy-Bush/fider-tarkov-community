ALTER TABLE notifications ADD COLUMN page_id INTEGER REFERENCES pages(id) ON DELETE CASCADE;
CREATE INDEX notifications_page ON notifications (page_id) WHERE page_id IS NOT NULL;

UPDATE notifications notification
SET page_id = page.id
FROM pages page
WHERE notification.tenant_id = page.tenant_id
  AND split_part(notification.link, '#', 1) = '/pages/' || page.slug;
