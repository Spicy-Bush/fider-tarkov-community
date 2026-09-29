package postgres_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestPostReceiptReturnsCurrentVisibleMetadata(t *testing.T) {
	f := newPostWorkflow(t)
	body := submissionBody(t, "current-metadata", false)
	created, err := f.request(api.CreatePost(), http.MethodPost, 0, body)
	if err != nil || created.Code != http.StatusOK {
		t.Fatalf("creation: status=%d error=%v", created.Code, err)
	}
	var original dto.PostSubmissionReceipt
	if err := json.Unmarshal(created.Body.Bytes(), &original); err != nil {
		t.Fatal(err)
	}

	if _, err := mediaFixtureSQL("UPDATE posts SET title='Current title',slug='current-slug' WHERE id=$1", original.ID); err != nil {
		t.Fatal(err)
	}
	replayed, err := f.request(api.CreatePost(), http.MethodPost, 0, body)
	if err != nil || replayed.Code != http.StatusOK {
		t.Fatalf("replay: status=%d error=%v", replayed.Code, err)
	}
	var current dto.PostSubmissionReceipt
	if err := json.Unmarshal(replayed.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current.ID != original.ID || current.Number != original.Number || current.Title != "Current title" || current.Slug != "current-slug" {
		t.Fatalf("receipt returned stale metadata: %+v", current)
	}

	if _, err := mediaFixtureSQL("UPDATE posts SET status=$1 WHERE id=$2", enum.PostDeleted, original.ID); err != nil {
		t.Fatal(err)
	}
	f.user.Muted = true
	replayed, err = f.request(api.CreatePost(), http.MethodPost, 0, body)
	if err != nil || replayed.Code != http.StatusOK {
		t.Fatalf("deleted replay: status=%d error=%v", replayed.Code, err)
	}
	if err := json.Unmarshal(replayed.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current.ID != original.ID || current.Number != original.Number || current.Title != "" || current.Slug != "" {
		t.Fatalf("deleted receipt exposed content or lost its accepted identity: %+v", current)
	}
	if count := workflowCount(t, "SELECT count(*) FROM posts"); count != 1 {
		t.Fatalf("receipt recovery created %d posts", count)
	}
}
