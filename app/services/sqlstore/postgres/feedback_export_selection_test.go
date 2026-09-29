package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/lib/pq"
)

func exportSelectionFixture(t testing.TB, count int) *dbx.Trx {
	t.Helper()
	trx, err := dbx.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = trx.Rollback() })

	for _, statement := range []string{
		"SET LOCAL statement_timeout = '45s'",
		"ALTER TABLE posts DISABLE TRIGGER USER",
		"UPDATE posts SET status = 6 WHERE tenant_id = 1",
	} {
		if _, err := trx.Execute(statement); err != nil {
			t.Fatal(err)
		}
	}

	_, err = trx.Execute(`
		INSERT INTO posts (
			tenant_id, title, slug, description, number, status, user_id,
			created_at, upvotes, downvotes, comments_count, moderation_pending
		)
		SELECT 1, 'Export selection ' || g, 'export-selection-' || g, 'Fixture',
			1000 + g, g % 6, 1, NOW() - make_interval(days => g % 100),
			g % 1000, g % 200, g % 50, g % 101 = 0
		FROM generate_series(1, $1::integer) g
	`, count)
	if err != nil {
		t.Fatal(err)
	}

	for _, statement := range []string{"ALTER TABLE posts ENABLE TRIGGER USER", "ANALYZE posts"} {
		if _, err := trx.Execute(statement); err != nil {
			t.Fatal(err)
		}
	}
	return trx
}

type exportSelection struct {
	Number int
	Mode   entity.FeedbackExportMode
}

func exportSequential(trx *dbx.Trx, recipe entity.FeedbackExportRecipe) ([][]exportSelection, error) {
	result := make([][]exportSelection, len(recipe.Sections))
	taken := []int64{}
	asOf := time.Now()
	for index, section := range recipe.Sections {
		result[index] = []exportSelection{}
		for _, pick := range section.Picks {
			ids, err := selectFeedbackExportPick(trx, 1, true, section, pick, "selection-seed", taken, asOf)
			if err != nil {
				return nil, err
			}

			for _, id := range ids {
				result[index] = append(result[index], exportSelection{Number: int(id), Mode: pick.Mode})
			}
			taken = append(taken, ids...)
		}
	}

	var posts []*struct {
		ID     int `db:"id"`
		Number int `db:"number"`
	}
	if err := trx.Select(&posts, "SELECT id, number FROM posts WHERE tenant_id = $1 AND id = ANY($2)", 1, pq.Array(taken)); err != nil {
		return nil, err
	}

	byID := make(map[int]int, len(posts))
	for _, post := range posts {
		byID[post.ID] = post.Number
	}

	for _, section := range result {
		for i := range section {
			section[i].Number = byID[section[i].Number]
		}
	}
	return result, nil
}

func exportPooled(trx *dbx.Trx, recipe entity.FeedbackExportRecipe) ([][]exportSelection, error) {
	q := &query.SelectFeedbackExportRows{Recipe: recipe, Seed: "selection-seed"}
	if err := readFeedbackExportRows(trx, 1, true, q); err != nil {
		return nil, err
	}

	result := make([][]exportSelection, len(q.Result))
	for i, rows := range q.Result {
		result[i] = []exportSelection{}
		for _, row := range rows {
			result[i] = append(result[i], exportSelection{Number: row.Number, Mode: row.Pick})
		}
	}
	return result, nil
}

func BenchmarkFeedbackExportHTTP100K(b *testing.B) {
	trx := exportSelectionFixture(b, 100000)
	if err := trx.Commit(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(dbx.Seed)
	bus.Init(Service{})
	engine := web.New()
	tenant := &entity.Tenant{ID: 1, Status: enum.TenantActive}
	user := &entity.User{ID: 1, Role: enum.RoleAdministrator, Status: enum.UserActive, Tenant: tenant}

	for _, workload := range []string{"top", "random", "mixed", "distinct_filters"} {
		mode := entity.FeedbackExportMode(workload)
		if workload == "mixed" || workload == "distinct_filters" {
			mode = entity.FeedbackExportRandom
		}

		recipe := entity.FeedbackExportRecipe{}
		for sectionIndex := 0; sectionIndex < 10; sectionIndex++ {
			section := entity.FeedbackExportSection{Statuses: entity.FeedbackExportStatuses}
			if workload == "distinct_filters" {
				section.MaxAgeDays = 100 - sectionIndex*5
				minVotes := sectionIndex * 20
				section.MinVotes = &minVotes
			}

			for pickIndex := 0; pickIndex < 5; pickIndex++ {
				pick := entity.FeedbackExportPick{Mode: mode, Count: 10}
				if workload == "mixed" && pickIndex%2 == 1 {
					pick.Mode = entity.FeedbackExportTopVoted
				}
				if workload == "distinct_filters" {
					pick.MinComments = pickIndex * 5
				}
				section.Picks = append(section.Picks, pick)
			}
			recipe.Sections = append(recipe.Sections, section)
		}

		body, err := json.Marshal(web.Map{"recipe": recipe, "seed": "selection-seed"})
		if err != nil {
			b.Fatal(err)
		}
		b.Run(workload, func(b *testing.B) {
			b.ReportAllocs()
			for iteration := 0; iteration < b.N; iteration++ {
				req := httptest.NewRequest(http.MethodPost, "/api/admin/bsg-export/preview", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				c, err := web.NewContext(engine, req, response, nil)
				if err != nil {
					b.Fatal(err)
				}
				c.SetTenant(tenant)
				c.SetUser(user)

				if err := handlers.PreviewFeedbackExport()(c); err != nil || response.Code != http.StatusOK {
					b.Fatalf("preview: %v, %d, %s", err, response.Code, response.Body)
				}
				if iteration == 0 {
					var result struct {
						Sections [][]entity.FeedbackExportRow `json:"sections"`
					}
					if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
						b.Fatal(err)
					}
					rows := 0
					for _, section := range result.Sections {
						rows += len(section)
					}
					if rows != 500 {
						b.Fatalf("preview returned %d rows, want 500", rows)
					}
				}
			}
		})
	}
}

func TestFeedbackExportCompatibleSelectionsMatchSequential(t *testing.T) {
	trx := exportSelectionFixture(t, 240)
	tagIDs := make([]int, 2)
	for i := range tagIDs {
		err := trx.Scalar(&tagIDs[i], `
			INSERT INTO tags (name, slug, color, is_public, created_at, tenant_id)
			VALUES ($1, $1, '123456', $2, NOW(), 1) RETURNING id
		`, fmt.Sprintf("export-selection-%d", i), i == 0)
		if err != nil {
			t.Fatal(err)
		}

		_, err = trx.Execute(`
			INSERT INTO post_tags (tag_id, post_id, created_at, created_by_id, tenant_id)
			SELECT $1, id, NOW(), 1, 1 FROM posts
			WHERE tenant_id = 1 AND number > 1000 AND number % $2 = 0
		`, tagIDs[i], i+2)
		if err != nil {
			t.Fatal(err)
		}
	}

	random := rand.New(rand.NewSource(928))
	modes := []entity.FeedbackExportMode{
		entity.FeedbackExportTopVoted,
		entity.FeedbackExportDiscussed,
		entity.FeedbackExportControversial,
		entity.FeedbackExportRandom,
	}

	for example := 0; example < 40; example++ {
		recipe := entity.FeedbackExportRecipe{}
		for sectionIndex := 0; sectionIndex < 10; sectionIndex++ {
			section := entity.FeedbackExportSection{Statuses: entity.FeedbackExportStatuses}
			if sectionIndex%3 == 0 {
				section.Statuses = []enum.PostStatus{enum.PostOpen, enum.PostStarted}
			}
			if sectionIndex%4 == 0 {
				section.MaxAgeDays = 90
			}
			if sectionIndex%3 == 1 {
				section.IncludeTags = []int{tagIDs[random.Intn(len(tagIDs))]}
			}
			if sectionIndex%3 == 2 {
				section.ExcludeTags = []int{tagIDs[random.Intn(len(tagIDs))]}
			}
			if sectionIndex%4 == 1 {
				minVotes := random.Intn(4) * 100
				section.MinVotes = &minVotes
			}

			for pickIndex := 0; pickIndex < 5; pickIndex++ {
				mode := modes[random.Intn(len(modes))]
				if example%2 == 0 {
					mode = entity.FeedbackExportRandom
				}

				section.Picks = append(section.Picks, entity.FeedbackExportPick{
					Mode:        mode,
					Count:       1 + random.Intn(10),
					MinComments: random.Intn(2) * 10,
				})
			}
			recipe.Sections = append(recipe.Sections, section)
		}

		expected, err := exportSequential(trx, recipe)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := exportPooled(trx, recipe)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("example %d changed selection, section order, or pick attribution:\nrecipe %+v\ngot %+v\nwant %+v", example, recipe, actual, expected)
		}
	}
}

func TestFeedbackExportCursorFailurePreservesErrorAndRecovers(t *testing.T) {
	trx := exportSelectionFixture(t, 240)
	q := &query.SelectFeedbackExportRows{
		Seed: "cursor-recovery",
		Recipe: entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{{
			Statuses: entity.FeedbackExportStatuses,
			Picks: []entity.FeedbackExportPick{
				{Mode: entity.FeedbackExportRandom, Count: 10},
				{Mode: entity.FeedbackExportRandom, Count: 10, MinComments: 5},
				{Mode: entity.FeedbackExportRandom, Count: 10, MinComments: 10},
			},
		}}},
	}

	_, err := trx.Execute(`
		SAVEPOINT export_failure;
		CREATE FUNCTION public.md5(text) RETURNS text LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'export fixture cancellation' USING ERRCODE = '57014';
		END;
		$$;
		SET LOCAL search_path = public, pg_catalog;
	`)
	if err != nil {
		t.Fatal(err)
	}

	err = readFeedbackExportRows(trx, 1, true, q)
	if errors.Cause(err) != context.Canceled || q.Result != nil {
		t.Fatalf("cancellation lost its cause or published a partial result: result %v, error %v", q.Result, err)
	}
	if !strings.Contains(err.Error(), "failed to read feedback export candidates") {
		t.Fatalf("failure did not reach the cursor read: %v", err)
	}

	if _, err := trx.Execute("ROLLBACK TO SAVEPOINT export_failure"); err != nil {
		t.Fatal(err)
	}
	if err := readFeedbackExportRows(trx, 1, true, q); err != nil {
		t.Fatal(err)
	}
	if len(q.Result[0]) != 30 {
		t.Fatalf("recovered selection has %d posts, want 30", len(q.Result[0]))
	}

	var cursors int
	if err := trx.Scalar(&cursors, "SELECT COUNT(*) FROM pg_cursors WHERE name LIKE 'feedback_export_%'"); err != nil || cursors != 0 {
		t.Fatalf("cursor survived completion: count %d, error %v", cursors, err)
	}
}
