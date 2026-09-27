CREATE FUNCTION visible_posts_for(viewer_tenant_id integer, viewer_role text, viewer_user_id integer)
RETURNS SETOF posts
LANGUAGE sql STABLE PARALLEL RESTRICTED AS $$
    SELECT p.*
    FROM posts p
    WHERE p.tenant_id = viewer_tenant_id
      AND p.status <> 6
      AND (
          NOT p.moderation_pending
          OR p.user_id = viewer_user_id
          OR viewer_role IN ('administrator', 'collaborator', 'moderator')
      );
$$;
