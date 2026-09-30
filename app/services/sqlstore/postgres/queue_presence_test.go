package postgres_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/sse"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func TestQueuePresenceRequiresVisiblePost(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Queue presence", Description: "Visible target"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	var foreignID, deletedID int
	err := dbx.Connection().QueryRow(`
		INSERT INTO posts (tenant_id, user_id, title, slug, description, created_at, status)
		SELECT tenant_id, id, 'Other tenant', 'other-tenant', '', NOW(), $2
		FROM users WHERE tenant_id <> $1 LIMIT 1 RETURNING id
	`, f.tenant.ID, enum.PostOpen).Scan(&foreignID)
	if err != nil {
		t.Fatal(err)
	}

	err = dbx.Connection().QueryRow(`
		INSERT INTO posts (tenant_id, user_id, title, slug, description, created_at, status)
		VALUES ($1, $2, 'Deleted post', 'deleted-post', '', NOW(), $3) RETURNING id
	`, f.tenant.ID, f.user.ID, enum.PostDeleted).Scan(&deletedID)
	if err != nil {
		t.Fatal(err)
	}

	client := sse.NewClient(f.tenant.ID, f.user.ID, f.user.Name, sse.ChannelQueue)
	hub := sse.GetHub()
	hub.Register(client)
	t.Cleanup(func() { hub.Unregister(client) })
	body := fmt.Sprintf(`{"connectionId":%q}`, client.ID())

	for _, id := range []int{post.Result.ID, -1, 0, 999999, foreignID, deletedID} {
		response, err := f.requestWithParams(handlers.QueuePostHeartbeat(), http.MethodPost, "/api/queue/heartbeat", body, web.StringMap{"id": fmt.Sprint(id)})
		want := http.StatusNotFound
		if id == post.Result.ID {
			want = http.StatusOK
		}
		if err != nil || response.Code != want {
			t.Fatalf("post %d: status=%d error=%v, wanted %d", id, response.Code, err, want)
		}
	}

	viewers := hub.GetAllQueueViewers(f.tenant.ID)
	if len(viewers) != 1 || viewers[0].PostID != post.Result.ID || len(viewers[0].Viewers) != 1 {
		t.Fatalf("rejected heartbeat changed presence: %+v", viewers)
	}
}
