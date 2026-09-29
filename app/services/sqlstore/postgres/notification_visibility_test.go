package postgres

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func visibilityFixture(t testing.TB) (context.Context, *dbx.Trx, int) {
	t.Helper()
	trx, err := dbx.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = trx.Rollback() })

	var postID, commentID int
	if err := trx.Get(&postID, `
        INSERT INTO posts (tenant_id,number,user_id,title,slug,description,status,created_at)
        VALUES (1,1001,1,'Visibility benchmark','visibility-benchmark','Description',1,now()) RETURNING id
    `); err != nil {
		t.Fatal(err)
	}
	if err := trx.Get(&commentID, `
        INSERT INTO comments (tenant_id,post_id,user_id,content,created_at,moderation_pending,moderation_data)
        VALUES (1,$1,1,repeat('Comment content ',2048),now(),true,'{"reason":"staff only"}') RETURNING id
    `, postID); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`
        INSERT INTO users(tenant_id,name,email,role,status,created_at,avatar_type,avatar_bkey)
        SELECT 1,'Reactor '||i,'reactor-'||i||'@visibility.test',1,1,now(),2,'' FROM generate_series(1,1000)i
    `); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`
        INSERT INTO reactions(comment_id,user_id,emoji,created_on)
        SELECT $1,id,'👍',now() FROM users WHERE email LIKE '%@visibility.test'
    `, commentID); err != nil {
		t.Fatal(err)
	}
	if _, err := trx.Execute(`
        INSERT INTO attachments(tenant_id,post_id,comment_id,user_id,attachment_bkey)
        SELECT 1,$1,$2,1,'image-'||i FROM generate_series(1,20)i
    `, postID, commentID); err != nil {
		t.Fatal(err)
	}

	requestURL, _ := url.Parse("https://test.fider.io:3000")
	ctx := context.WithValue(context.Background(), app.RequestCtxKey, web.Request{URL: requestURL})
	ctx = context.WithValue(ctx, app.TransactionCtxKey, trx)
	ctx = context.WithValue(ctx, app.TenantCtxKey, &entity.Tenant{ID: 1})
	ctx = context.WithValue(ctx, app.UserCtxKey, &entity.User{ID: 2, Role: enum.RoleVisitor, Status: enum.UserActive})
	return ctx, trx, commentID
}

func TestNotificationVisibilityMatchesCommentProjection(t *testing.T) {
	ctx, trx, id := visibilityFixture(t)
	roles := []enum.Role{enum.RoleVisitor, enum.RoleHelper, enum.RoleModerator, enum.RoleCollaborator, enum.RoleAdministrator}
	discussions := []*entity.Discussion{
		entity.PostDiscussion(&entity.Post{Status: enum.PostOpen}),
		entity.PageDiscussion(&entity.Page{Status: entity.PageStatusPublished, Visibility: entity.PageVisibilityPublic}),
		entity.PageDiscussion(&entity.Page{Status: entity.PageStatusPublished, Visibility: entity.PageVisibilityPrivate}),
	}
	checks := 0
	for _, authorRole := range roles {
		if _, err := trx.Execute("UPDATE users SET role=$1 WHERE id=1", authorRole); err != nil {
			t.Fatal(err)
		}
		for _, hidden := range []bool{false, true} {
			for _, deleted := range []bool{false, true} {
				if _, err := trx.Execute(`
                    UPDATE comments SET moderation_pending=$1,
                    deleted_at=CASE WHEN $2 THEN now() ELSE NULL END WHERE id=$3
                `, hidden, deleted, id); err != nil {
					t.Fatal(err)
				}
				full := &query.GetCommentByID{CommentID: id, IncludeDeleted: true}
				if err := getCommentByID(ctx, full); err != nil {
					t.Fatal(err)
				}
				if full.Result.ModerationData == "" {
					t.Fatal("storage hydration discarded facts based on viewer")
				}
				focused, err := readCommentVisibility(trx, 1, id)
				if err != nil {
					t.Fatal(err)
				}
				for _, role := range roles {
					for _, own := range []bool{false, true} {
						viewer := &entity.User{ID: 2, Role: role, Status: enum.UserActive}
						if own {
							viewer.ID = 1
						}
						for _, discussion := range discussions {
							projection := full.Result.ForViewer(viewer, discussion, &entity.Tenant{}, time.Now())
							if focused.ContentState(viewer, discussion, &entity.Tenant{}) != projection.State {
								t.Fatal("focused visibility and response projection disagree")
							}
							if !projection.Permissions.Moderate && (projection.ModerationData != "" || projection.ModerationPending) {
								t.Fatal("response disclosed moderation to ordinary viewer")
							}
							checks++
						}
					}
				}
			}
		}
	}
	t.Logf("matched %d role/owner/status projections", checks)
}

func BenchmarkNotificationVisibilityData(b *testing.B) {
	ctx, trx, id := visibilityFixture(b)
	b.Run("full", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			q := &query.GetCommentByID{CommentID: id, IncludeDeleted: true}
			if err := getCommentByID(ctx, q); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("focused", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := readCommentVisibility(trx, 1, id); err != nil {
				b.Fatal(err)
			}
		}
	})
}
