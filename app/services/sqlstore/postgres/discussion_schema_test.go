package postgres_test

import (
	"errors"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/lib/pq"
)

func TestDiscussionSchemaConstraints(t *testing.T) {
	newPostWorkflow(t)

	_, err := dbx.Connection().Exec(`
        INSERT INTO posts (id, tenant_id, user_id, number, title, slug, status, created_at)
        VALUES (1, 1, 1, 1, 'First post', 'first', 1, NOW()),
               (2, 1, 1, 2, 'Second post', 'second', 1, NOW()),
               (3, 2, 4, 1, 'Other tenant', 'other', 1, NOW());

        INSERT INTO pages (id, tenant_id, created_by_id, updated_by_id, title, slug, content)
        VALUES (1, 1, 1, 1, 'First Page', 'first', 'Content'),
               (2, 1, 1, 1, 'Second Page', 'second', 'Content'),
               (3, 2, 4, 4, 'Other tenant', 'other', 'Content');

        INSERT INTO comments (id, tenant_id, user_id, post_id, page_id, parent_id, submission_id, content, created_at)
        VALUES (1, 1, 1, 1, NULL, NULL, 'original', 'Post root', NOW()),
               (2, 1, 1, 1, NULL, 1, 'reply', 'Post reply', NOW()),
               (3, 1, 1, NULL, 1, NULL, 'page', 'Page root', NOW()),
               (4, 1, 1, NULL, 1, 3, 'page-reply', 'Page reply', NOW()),
               (5, 2, 4, 3, NULL, NULL, 'original', 'Other tenant post', NOW()),
               (6, 2, 4, NULL, 3, NULL, 'page', 'Other tenant Page', NOW()),
               (7, 1, 2, 1, NULL, NULL, 'original', 'Other author', NOW()),
               (8, 1, 1, 1, NULL, NULL, NULL, 'Existing root', NOW()),
               (9, 1, 1, 1, NULL, NULL, NULL, 'Another existing root', NOW());

        INSERT INTO attachments (tenant_id, user_id, post_id, comment_id, attachment_bkey)
        VALUES (1, 1, 1, 2, 'attachments/post-reply.png'),
               (1, 1, NULL, 4, 'attachments/page-reply.png'),
               (1, 1, 1, NULL, 'attachments/post.png');

        INSERT INTO reactions (comment_id, user_id, emoji, created_on)
        VALUES (2, 1, '👍', NOW()), (4, 1, '👍', NOW());
    `)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		sql  string
		code pq.ErrorCode
	}{
		{
			"parent in another post",
			`INSERT INTO comments (id, tenant_id, user_id, post_id, parent_id, created_at) VALUES (20, 1, 1, 2, 1, NOW())`,
			"23503",
		},
		{
			"parent in another Page",
			`INSERT INTO comments (id, tenant_id, user_id, page_id, parent_id, created_at) VALUES (20, 1, 1, 2, 3, NOW())`,
			"23503",
		},
		{
			"parent in another tenant",
			`INSERT INTO comments (id, tenant_id, user_id, post_id, parent_id, created_at) VALUES (20, 2, 4, 3, 1, NOW())`,
			"23503",
		},
		{
			"post reply to Page comment",
			`INSERT INTO comments (id, tenant_id, user_id, post_id, parent_id, created_at) VALUES (20, 1, 1, 1, 3, NOW())`,
			"23503",
		},
		{
			"parent after child",
			`INSERT INTO comments (id, tenant_id, user_id, post_id, parent_id, created_at) VALUES (0, 1, 1, 1, 1, NOW())`,
			"23514",
		},
		{
			"self reply",
			`INSERT INTO comments (id, tenant_id, user_id, post_id, parent_id, created_at) VALUES (20, 1, 1, 1, 20, NOW())`,
			"23514",
		},
		{"change parent", `UPDATE comments SET parent_id = NULL WHERE id = 2`, "P0001"},
		{"change post", `UPDATE comments SET post_id = 2 WHERE id = 1`, "P0001"},
		{"change Page", `UPDATE comments SET page_id = 2 WHERE id = 3`, "P0001"},
		{"change tenant", `UPDATE comments SET tenant_id = 2, user_id = 4, post_id = 3 WHERE id = 1`, "P0001"},
		{
			"duplicate submission",
			`INSERT INTO comments (id, tenant_id, user_id, post_id, submission_id, created_at) VALUES (20, 1, 1, 2, 'original', NOW())`,
			"23505",
		},
		{
			"attachment without owner",
			`INSERT INTO attachments (tenant_id, user_id, attachment_bkey) VALUES (1, 1, 'attachments/orphan.png')`,
			"23514",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			transaction, err := dbx.Connection().Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer transaction.Rollback()

			_, err = transaction.Exec(test.sql)
			var constraint *pq.Error
			if !errors.As(err, &constraint) || constraint.Code != test.code {
				t.Fatalf("expected database error %s, got %v", test.code, err)
			}
		})
	}

	if _, err := dbx.Connection().Exec("UPDATE comments SET content = 'Edited', moderation_pending = TRUE WHERE id = 2"); err != nil {
		t.Fatalf("ordinary comment edit rejected: %v", err)
	}

	if workflowCount(t, "SELECT COUNT(*) FROM pages WHERE allow_comment_images") != 0 {
		t.Fatal("Page comment images were enabled without an explicit setting")
	}

	if _, err := dbx.Connection().Exec("DELETE FROM comments WHERE id IN (1, 3)"); err != nil {
		t.Fatal(err)
	}

	if workflowCount(t, "SELECT COUNT(*) FROM comments") != 5 ||
		workflowCount(t, "SELECT COUNT(*) FROM comments WHERE id IN (1, 2, 3, 4)") != 0 ||
		workflowCount(t, "SELECT COUNT(*) FROM attachments") != 1 ||
		workflowCount(t, "SELECT COUNT(*) FROM attachments WHERE comment_id IS NOT NULL") != 0 ||
		workflowCount(t, "SELECT COUNT(*) FROM reactions") != 0 {
		t.Fatal("deleting threads retained dependent records or removed unrelated data")
	}
}
