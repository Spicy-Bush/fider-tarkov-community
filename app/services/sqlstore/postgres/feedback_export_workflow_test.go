package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/handlers"
	"github.com/Spicy-Bush/fider-tarkov-community/app/middlewares"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
)

func exportPresetBody(id, name string, count int) string {
	return fmt.Sprintf(`{"id":%q,"name":%q,"recipe":{"sections":[{"name":"Feedback","statuses":["open"],"picks":[{"mode":"top","count":%d}]}]}}`, id, name, count)
}

func exportPresetUpdateBody(name string, count int, saved *entity.FeedbackExportPreset) string {
	var body map[string]any
	_ = json.Unmarshal([]byte(exportPresetBody("", name, count)), &body)
	body["submissionId"] = fmt.Sprintf("%s:%d:%s", name, count, saved.UpdatedAt.Format("15:04:05.999999999"))
	body["saved"] = entity.FeedbackExportPresetContent{Name: saved.Name, Recipe: saved.Recipe}
	encoded, _ := json.Marshal(body)
	return string(encoded)
}

func TestFeedbackExportPresetEditsIgnoreFilterSetOrder(t *testing.T) {
	f := newPostWorkflow(t)
	id := strings.Repeat("e", 32)
	stored := `{"sections":[{"name":"Feedback","statuses":["planned","open","planned"],"includeTags":[2,1,2],"excludeTags":[5,4,5],"picks":[{"mode":"top","count":10}]}]}`
	_, err := dbx.Connection().Exec(`
		INSERT INTO feedback_export_presets (tenant_id, id, name, recipe, created_at, updated_at, updated_by_id)
		VALUES ($1, $2, 'Original', $3, NOW(), NOW(), $4)
	`, f.tenant.ID, id, stored, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}

	presets := &query.ListFeedbackExportPresets{}
	if err := bus.Dispatch(f.ctx, presets); err != nil {
		t.Fatal(err)
	}
	baseline := presets.Result[0]

	for _, count := range []int{10, 20} {
		var desired entity.FeedbackExportRecipe
		if err := json.Unmarshal([]byte(stored), &desired); err != nil {
			t.Fatal(err)
		}
		desired.Sections[0].Picks[0].Count = count
		body, err := json.Marshal(struct {
			SubmissionID string                             `json:"submissionId"`
			Name         string                             `json:"name"`
			Recipe       entity.FeedbackExportRecipe        `json:"recipe"`
			Saved        entity.FeedbackExportPresetContent `json:"saved"`
		}{
			SubmissionID: fmt.Sprintf("normalize-%d", count),
			Name:         "Renamed",
			Recipe:       desired,
			Saved:        entity.FeedbackExportPresetContent{Name: baseline.Name, Recipe: baseline.Recipe},
		})
		if err != nil {
			t.Fatal(err)
		}

		response, err := f.requestWithParams(middlewares.WebSetup()(handlers.UpdateFeedbackExportPreset()), http.MethodPut,
			"http://localhost:3000/api/admin/bsg-export/presets/"+id, string(body), web.StringMap{"id": id})
		if err != nil || response.Code != http.StatusOK {
			t.Fatalf("count %d: status %d, error %v, body %s", count, response.Code, err, response.Body)
		}

		var update entity.FeedbackExportPresetUpdate
		if err := json.Unmarshal(response.Body.Bytes(), &update); err != nil {
			t.Fatal(err)
		}
		if len(update.Conflicts) != 0 || update.Preset.Name != "Renamed" || update.Preset.Recipe.Sections[0].Picks[0].Count != count {
			t.Fatalf("filter set ordering blocked count %d: %+v", count, update)
		}
		baseline = update.Preset
	}
}

func TestFeedbackExportPresetHTTPRecovery(t *testing.T) {
	f := newPostWorkflow(t)
	id := strings.Repeat("1", 32)
	otherID := strings.Repeat("2", 32)
	original := exportPresetBody(id, "Original", 10)

	request := func(handler web.HandlerFunc, method, target, body string, status int) *entity.FeedbackExportPreset {
		t.Helper()
		response, err := f.requestWithParams(middlewares.WebSetup()(handler), method,
			"http://localhost:3000/api/admin/bsg-export/presets/"+target, body, web.StringMap{"id": target})
		if err != nil || response.Code != status {
			t.Fatalf("%s %s: status %d, error %v, body %s", method, target, response.Code, err, response.Body)
		}

		var preset entity.FeedbackExportPreset
		if status == http.StatusOK && method != http.MethodDelete {
			if method == http.MethodPut {
				var update entity.FeedbackExportPresetUpdate
				if err := json.Unmarshal(response.Body.Bytes(), &update); err != nil {
					t.Fatal(err)
				}
				if len(update.Conflicts) > 0 {
					t.Fatalf("unexpected conflict: %+v", update)
				}
				return update.Preset
			}

			if err := json.Unmarshal(response.Body.Bytes(), &preset); err != nil {
				t.Fatal(err)
			}
		}
		return &preset
	}

	created := request(handlers.CreateFeedbackExportPreset(), http.MethodPost, "", original, http.StatusOK)
	if created.Recipe.Sections[0].IncludeTags == nil || created.Recipe.Sections[0].ExcludeTags == nil {
		t.Fatal("omitted tag filters must be returned as arrays")
	}

	renamed := request(handlers.UpdateFeedbackExportPreset(), http.MethodPut, id,
		exportPresetUpdateBody("Renamed", 20, created), http.StatusOK)
	if renamed.Recipe.Sections[0].IncludeTags == nil || renamed.Recipe.Sections[0].ExcludeTags == nil {
		t.Fatal("updated tag filters must be returned as arrays")
	}
	request(handlers.CreateFeedbackExportPreset(), http.MethodPost, "",
		exportPresetBody(otherID, "Original", 30), http.StatusOK)

	replayed := request(handlers.CreateFeedbackExportPreset(), http.MethodPost, "", original, http.StatusOK)
	if replayed.ID != id || replayed.Name != "Renamed" || replayed.Recipe.Sections[0].Picks[0].Count != 20 {
		t.Fatalf("create replay replaced accepted edits: %+v", replayed)
	}

	for _, malformed := range []string{"", "0", "not-an-id", strings.Repeat("9", 40)} {
		request(handlers.UpdateFeedbackExportPreset(), http.MethodPut, malformed, original, http.StatusNotFound)
	}
	if count := workflowCount(t, "SELECT COUNT(*) FROM feedback_export_presets WHERE tenant_id = $1", f.tenant.ID); count != 2 {
		t.Fatalf("malformed update created a preset: %d rows", count)
	}

	for _, wrapped := range []bool{false, true} {
		handler := handlers.UpdateFeedbackExportPreset()
		if wrapped {
			handler = func(c *web.Context) error {
				return c.WithTransaction(func() error { return handlers.UpdateFeedbackExportPreset()(c) })
			}
		}
		request(handler, http.MethodPut, id, exportPresetUpdateBody("ORIGINAL", 40, renamed), http.StatusBadRequest)
	}

	replayed = request(handlers.CreateFeedbackExportPreset(), http.MethodPost, "", original, http.StatusOK)
	if replayed.Name != "Renamed" || replayed.Recipe.Sections[0].Picks[0].Count != 20 {
		t.Fatalf("failed rename changed the preset: %+v", replayed)
	}
	request(handlers.UpdateFeedbackExportPreset(), http.MethodPut, id,
		exportPresetUpdateBody("Recovered", 50, renamed), http.StatusOK)

	request(handlers.DeleteFeedbackExportPreset(), http.MethodDelete, id, "", http.StatusOK)
	request(handlers.DeleteFeedbackExportPreset(), http.MethodDelete, id, "", http.StatusOK)
	request(handlers.CreateFeedbackExportPreset(), http.MethodPost, "", original, http.StatusNotFound)
	request(handlers.UpdateFeedbackExportPreset(), http.MethodPut, id,
		exportPresetUpdateBody("Renamed", 20, created), http.StatusNotFound)

	var cleared bool
	if err := dbx.Connection().QueryRow(`
		SELECT name IS NULL AND recipe IS NULL AND updated_by_id IS NULL
		FROM feedback_export_presets WHERE tenant_id = $1 AND id = $2
	`, f.tenant.ID, id).Scan(&cleared); err != nil || !cleared {
		t.Fatalf("deleted identity retained its payload: cleared %v, error %v", cleared, err)
	}
}

func TestFeedbackExportPresetConcurrentCreation(t *testing.T) {
	for _, sameIdentity := range []bool{true, false} {
		t.Run(fmt.Sprintf("same_identity_%v", sameIdentity), func(t *testing.T) {
			f := newPostWorkflow(t)
			ids := []string{strings.Repeat("a", 32), strings.Repeat("b", 32)}
			if sameIdentity {
				ids[1] = ids[0]
			}

			statuses := make([]int, 2)
			errors := make([]error, 2)
			var workers sync.WaitGroup
			for i, id := range ids {
				workers.Add(1)
				go func(index int, id string) {
					defer workers.Done()
					response, err := f.requestWithParams(handlers.CreateFeedbackExportPreset(), http.MethodPost,
						"http://localhost:3000/api/admin/bsg-export/presets", exportPresetBody(id, "Concurrent", 10), nil)
					statuses[index], errors[index] = response.Code, err
				}(i, id)
			}
			workers.Wait()

			if errors[0] != nil || errors[1] != nil {
				t.Fatalf("concurrent create errors: %v", errors)
			}
			if sameIdentity && (statuses[0] != 200 || statuses[1] != 200) {
				t.Fatalf("same identity did not converge: %v", statuses)
			}
			if !sameIdentity && statuses[0]+statuses[1] != 600 {
				t.Fatalf("conflicting names did not produce one 200 and one 400: %v", statuses)
			}
			if count := workflowCount(t, "SELECT COUNT(*) FROM feedback_export_presets WHERE tenant_id = $1", f.tenant.ID); count != 1 {
				t.Fatalf("concurrent create stored %d presets", count)
			}
		})
	}
}

func TestFeedbackExportPresetUpdateReconciliation(t *testing.T) {
	f := newPostWorkflow(t)
	id := strings.Repeat("c", 32)
	response, err := f.requestWithParams(handlers.CreateFeedbackExportPreset(), http.MethodPost,
		"http://localhost:3000/api/admin/bsg-export/presets", exportPresetBody(id, "Weekly", 10), nil)
	if err != nil || response.Code != http.StatusOK {
		t.Fatalf("create: %v, %d, %s", err, response.Code, response.Body)
	}

	var original entity.FeedbackExportPreset
	if err := json.Unmarshal(response.Body.Bytes(), &original); err != nil {
		t.Fatal(err)
	}

	update := func(name string, count int, saved *entity.FeedbackExportPreset) entity.FeedbackExportPresetUpdate {
		t.Helper()
		response, err := f.requestWithParams(handlers.UpdateFeedbackExportPreset(), http.MethodPut,
			"http://localhost:3000/api/admin/bsg-export/presets/"+id,
			exportPresetUpdateBody(name, count, saved), web.StringMap{"id": id})
		if err != nil || response.Code != http.StatusOK {
			t.Fatalf("update: %v, %d, %s", err, response.Code, response.Body)
		}

		var result entity.FeedbackExportPresetUpdate
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}

	rename := update("Renamed", 10, &original)
	merged := update("Weekly", 20, &original)
	if len(merged.Conflicts) != 0 || merged.Preset.Name != "Renamed" || merged.Preset.Recipe.Sections[0].Picks[0].Count != 20 {
		t.Fatalf("independent edits did not merge: %+v", merged)
	}

	replayed := update("Renamed", 10, &original)
	if !reflect.DeepEqual(replayed, merged) {
		t.Fatalf("lost acknowledgement retry changed independent work: got %+v, want %+v", replayed, merged)
	}

	newer := update("Newest", 30, merged.Preset)
	conflict := update("Renamed", 10, &original)
	if len(conflict.Conflicts) != 0 || !reflect.DeepEqual(conflict.Preset, newer.Preset) {
		t.Fatalf("late retry overwrote a newer rename: %+v", conflict)
	}

	conflict = update("Original local", 40, rename.Preset)
	if !reflect.DeepEqual(conflict.Conflicts, []string{"name", "recipe"}) || !reflect.DeepEqual(conflict.Preset, newer.Preset) {
		t.Fatalf("same-field conflict lost its current state: %+v", conflict)
	}

	resolved := update("Original local", 40, conflict.Preset)
	if len(resolved.Conflicts) != 0 || resolved.Preset.Name != "Original local" || resolved.Preset.Recipe.Sections[0].Picks[0].Count != 40 {
		t.Fatalf("explicit conflict resolution failed: %+v", resolved)
	}

	restored := update("Weekly", 10, resolved.Preset)
	if len(restored.Conflicts) != 0 {
		t.Fatalf("restoring the original values failed: %+v", restored)
	}

	for _, test := range []struct {
		name     string
		count    int
		conflict string
	}{
		{name: "Renamed", count: 10, conflict: "name"},
		{name: "Weekly", count: 20, conflict: "recipe"},
	} {
		retry := update(test.name, test.count, &original)
		if len(retry.Conflicts) != 0 || !reflect.DeepEqual(retry.Preset, restored.Preset) {
			t.Fatalf("retry overwrote a later restoration: %+v", retry)
		}
	}

	unsent := update("Fresh intent", 10, &original)
	if len(unsent.Conflicts) != 0 || unsent.Preset.Name != "Fresh intent" {
		t.Fatal("first save rejected an unchanged baseline after unrelated history")
	}

	if _, err := dbx.Connection().Exec("UPDATE users SET status = $1 WHERE id = $2", enum.UserBlocked, f.user.ID); err != nil {
		t.Fatal(err)
	}
	response, err = f.requestWithParams(handlers.UpdateFeedbackExportPreset(), http.MethodPut,
		"http://localhost:3000/api/admin/bsg-export/presets/"+id,
		exportPresetUpdateBody("Renamed", 10, &original), web.StringMap{"id": id})
	if err != nil || response.Code != http.StatusForbidden {
		t.Fatalf("receipt bypassed actor revocation: status=%d error=%v", response.Code, err)
	}
}

func TestFeedbackExportPresetConcurrentUpdates(t *testing.T) {
	for _, sameField := range []bool{false, true} {
		t.Run(fmt.Sprintf("same_field_%v", sameField), func(t *testing.T) {
			f := newPostWorkflow(t)
			id := strings.Repeat("d", 32)
			response, err := f.requestWithParams(handlers.CreateFeedbackExportPreset(), http.MethodPost,
				"http://localhost:3000/api/admin/bsg-export/presets", exportPresetBody(id, "Original", 10), nil)
			if err != nil || response.Code != http.StatusOK {
				t.Fatalf("create: %v, %d, %s", err, response.Code, response.Body)
			}

			var original entity.FeedbackExportPreset
			if err := json.Unmarshal(response.Body.Bytes(), &original); err != nil {
				t.Fatal(err)
			}

			bodies := []string{
				exportPresetUpdateBody("Renamed", 10, &original),
				exportPresetUpdateBody("Original", 20, &original),
			}
			if sameField {
				bodies[1] = exportPresetUpdateBody("Other name", 10, &original)
			}

			results := make([]entity.FeedbackExportPresetUpdate, 2)
			errors := make([]error, 2)
			var workers sync.WaitGroup
			for i, body := range bodies {
				workers.Add(1)
				go func(index int, body string) {
					defer workers.Done()
					response, err := f.requestWithParams(handlers.UpdateFeedbackExportPreset(), http.MethodPut,
						"http://localhost:3000/api/admin/bsg-export/presets/"+id, body, web.StringMap{"id": id})
					if err != nil || response.Code != http.StatusOK {
						errors[index] = fmt.Errorf("update: %v, %d, %s", err, response.Code, response.Body)
						return
					}
					errors[index] = json.Unmarshal(response.Body.Bytes(), &results[index])
				}(i, body)
			}
			workers.Wait()

			if errors[0] != nil || errors[1] != nil {
				t.Fatalf("concurrent updates: %v", errors)
			}
			conflicts := len(results[0].Conflicts) + len(results[1].Conflicts)
			if sameField && conflicts != 1 || !sameField && conflicts != 0 {
				t.Fatalf("unexpected conflicts: %+v", results)
			}

			var name, recipe string
			if err := dbx.Connection().QueryRow("SELECT name, recipe FROM feedback_export_presets WHERE tenant_id = $1 AND id = $2", f.tenant.ID, id).Scan(&name, &recipe); err != nil {
				t.Fatal(err)
			}
			var persisted entity.FeedbackExportRecipe
			if err := json.Unmarshal([]byte(recipe), &persisted); err != nil {
				t.Fatal(err)
			}
			if !sameField && (name != "Renamed" || persisted.Sections[0].Picks[0].Count != 20) {
				t.Fatalf("concurrent independent edits lost: %s %s", name, recipe)
			}
		})
	}
}

func BenchmarkFeedbackExportPresetSaveHTTP(b *testing.B) {
	f := newPostWorkflow(b)
	id := strings.Repeat("c", 32)
	response, err := f.requestWithParams(handlers.CreateFeedbackExportPreset(), http.MethodPost,
		"http://localhost:3000/api/admin/bsg-export/presets", exportPresetBody(id, "Weekly", 10), nil)
	if err != nil || response.Code != http.StatusOK {
		b.Fatalf("create: %v, status %d", err, response.Code)
	}

	var saved entity.FeedbackExportPreset
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := exportPresetUpdateBody("Weekly", 10+i%2, &saved)
		response, err := f.requestWithParams(handlers.UpdateFeedbackExportPreset(), http.MethodPut,
			"http://localhost:3000/api/admin/bsg-export/presets/"+id, body, web.StringMap{"id": id})
		if err != nil || response.Code != http.StatusOK {
			b.Fatalf("save: %v, status %d", err, response.Code)
		}

		var update entity.FeedbackExportPresetUpdate
		if err := json.Unmarshal(response.Body.Bytes(), &update); err != nil {
			b.Fatal(err)
		}
		if len(update.Conflicts) != 0 || update.Preset.Recipe.Sections[0].Picks[0].Count != 10+i%2 {
			b.Fatalf("save did not complete: %+v", update)
		}
		saved = *update.Preset
	}
}
