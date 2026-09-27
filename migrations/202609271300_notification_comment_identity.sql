UPDATE notifications notification
SET comment_id = comment.id,
    page_id = comment.page_id,
    link = CASE WHEN post.id IS NOT NULL
        THEN '/posts/' || post.number || '/' || post.slug
        ELSE '/pages/' || page.slug
    END || '#comment-' || comment.id
FROM comments comment
LEFT JOIN posts post ON post.id = comment.post_id AND post.tenant_id = comment.tenant_id
LEFT JOIN pages page ON page.id = comment.page_id AND page.tenant_id = comment.tenant_id
WHERE notification.comment_id IS NULL
  AND comment.tenant_id = notification.tenant_id
  AND comment.id::text = substring(notification.link FROM '#comment-([0-9]+)$')
  AND (
    (notification.post_id = post.id
      AND notification.link ~ '^/posts/[0-9]+/[^#]+#comment-[0-9]+$')
    OR (notification.post_id IS NULL AND page.id IS NOT NULL
      AND notification.link ~ '^/pages/[^#]+#comment-[0-9]+$')
  );

-- Legacy content notifications need a verifiable owner before display.
DELETE FROM notifications
WHERE (comment_id IS NULL
    AND link ~ '^/(posts/[0-9]+|pages)/[^#]+#comment-[0-9]+$')
  OR (page_id IS NULL AND post_id IS NULL AND link ~ '^/pages/[^#]+$');
