package postgres_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers/api"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/rand"
)

func TestSponsorAPIRequiresIssuedOpportunities(t *testing.T) {
	f := newPostWorkflow(t)
	post := &cmd.AddNewPost{Title: "Restored feed post", Description: "Visible after returning to the feed"}
	if err := bus.Dispatch(f.ctx, post); err != nil {
		t.Fatal(err)
	}

	for _, placement := range entity.SponsorPlacements {
		if placement.ID == "strip_desktop" || placement.ID == "feed_desktop" {
			placement.Enabled = true
			placement.Empty = "none"
			if err := bus.Dispatch(f.ctx, &cmd.SaveSponsorPlacement{SubmissionID: rand.String(32), Placement: placement}); err != nil {
				t.Fatal(err)
			}
		}
	}
	f.user = nil

	page := adsselect.Page{ID: rand.String(32), TenantID: f.tenant.ID, Session: adsselect.SessionKey(""), Kind: "home", PostIDs: []int{7, 15}, Expires: time.Now().Add(time.Hour).Unix()}
	token, err := page.Token(env.Config.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	continuation := page
	continuation.PostIDs = []int{23, 31}
	continued, err := continuation.Token(env.Config.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name         string
		token        string
		placement    string
		instance     string
		continuation string
		status       int
		receipts     int
	}{
		{"missing page", "", "strip_desktop", "strip_desktop", "", 403, 0},
		{"unissued page", "invalid", "strip_desktop", "strip_desktop", "", 403, 0},
		{"invented instance", token, "strip_desktop", "another-strip", "", 403, 0},
		{"unserved feed", token, "feed_desktop", "feed-2", "", 403, 0},
		{"anonymous page", token, "strip_desktop", "strip_desktop", "", 200, 1},
		{"retried page", token, "strip_desktop", "strip_desktop", "", 200, 1},
		{"visible feed", token, "feed_desktop", "feed-7", "", 200, 2},
		{"later feed", token, "feed_desktop", "feed-23", continued, 200, 3},
		{"retried feed", token, "feed_desktop", "feed-23", continued, 200, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{
				"pageToken":     test.token,
				"context":       entity.SponsorContext{PageType: "home", Language: "en", Device: "desktop"},
				"opportunities": []entity.SponsorOpportunity{{InstanceID: test.instance, PlacementID: test.placement, PageToken: test.continuation}},
			})
			if err != nil {
				t.Fatal(err)
			}
			response, err := f.requestWithParams(api.AllocateSponsors(), http.MethodPost, "/api/sponsorship/select", string(body), nil)
			if err != nil || response.Code != test.status {
				t.Fatalf("status=%d error=%v body=%s", response.Code, err, response.Body.String())
			}
			if count := workflowCount(t, "SELECT count(*) FROM sponsor_opportunities"); count != test.receipts {
				t.Fatalf("created %d receipts, wanted %d", count, test.receipts)
			}
		})
	}

	search := url.Values{"ids": {strconv.Itoa(post.Result.ID)}, "sponsorPage": {token}}
	response, err := f.requestWithParams(api.SearchPosts(), http.MethodGet, "/api/posts?"+search.Encode(), "", nil)
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("restoring posts: status=%d error=%v", response.Code, err)
	}

	var restored []*entity.Post
	if err := json.Unmarshal(response.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored) != 1 || restored[0].SponsorPage == "" {
		t.Fatalf("restored posts lack sponsorship grants: %+v", restored)
	}

	body, err := json.Marshal(map[string]any{
		"pageToken": token,
		"context":   entity.SponsorContext{PageType: "home", Language: "en", Device: "desktop"},
		"opportunities": []entity.SponsorOpportunity{{
			InstanceID:  "feed-" + strconv.Itoa(post.Result.ID),
			PlacementID: "feed_desktop", PageToken: restored[0].SponsorPage,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err = f.requestWithParams(api.AllocateSponsors(), http.MethodPost, "/api/sponsorship/select", string(body), nil)
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("restored feed: status=%d error=%v body=%s", response.Code, err, response.Body.String())
	}

	if _, err := dbx.Connection().Exec("UPDATE sponsor_opportunities SET expires_at=NOW()-INTERVAL '1 second'"); err != nil {
		t.Fatal(err)
	}
	if err := bus.Dispatch(f.ctx, &cmd.PurgeSponsorOpportunities{}); err != nil {
		t.Fatal(err)
	}
	if count := workflowCount(t, "SELECT count(*) FROM sponsor_opportunities"); count != 0 {
		t.Fatalf("retained %d expired receipts", count)
	}
}
