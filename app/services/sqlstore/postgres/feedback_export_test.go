package postgres_test

import (
	"context"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"

	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
)

type exportFixture struct {
	ctx                context.Context
	bug, qol, arena    int
	numbers            map[string]int
	numberTitles       map[int]string
	openPlannedStarted []enum.PostStatus
}

func newExportFixture(t *testing.T, ctx context.Context) exportFixture {
	f := exportFixture{
		ctx:                withTenant(withUser(ctx, jonSnow), demoTenant),
		numbers:            map[string]int{},
		numberTitles:       map[int]string{},
		openPlannedStarted: []enum.PostStatus{enum.PostOpen, enum.PostPlanned, enum.PostStarted},
	}

	tag := func(name string) int {
		add := &cmd.AddNewTag{Name: name, Color: "FF0000", IsPublic: true}
		Expect(bus.Dispatch(f.ctx, add)).IsNil()
		return add.Result.ID
	}
	f.bug, f.qol, f.arena = tag("Bug"), tag("QoL"), tag("Arena")

	for _, post := range []struct {
		title              string
		up, down, comments int
		status             enum.PostStatus
		ageDays            int
		pending            bool
		tags               []int
	}{
		{"A", 100, 0, 5, enum.PostOpen, 1, false, []int{f.bug}},
		{"B", 80, 70, 40, enum.PostOpen, 1, false, []int{f.bug}},
		{"C", 60, 0, 1, enum.PostPlanned, 1, false, []int{f.qol}},
		{"D", 50, 45, 2, enum.PostOpen, 1, false, []int{f.arena}},
		{"E", 40, 0, 30, enum.PostCompleted, 1, false, []int{f.bug}},
		{"F", 30, 0, 12, enum.PostOpen, 400, false, []int{f.bug, f.arena}},
		{"G", 20, 0, 8, enum.PostOpen, 1, false, nil},
		{"H", 200, 0, 50, enum.PostDeleted, 1, false, []int{f.bug}},
		{"I", 150, 0, 0, enum.PostOpen, 1, true, []int{f.bug}},
		{"J", 10, 0, 3, enum.PostArchived, 1, false, []int{f.bug}},
	} {
		add := &cmd.AddNewPost{Title: "Export fixture " + post.title, Description: "Fixture"}
		Expect(bus.Dispatch(f.ctx, add)).IsNil()
		_, err := trx.Execute(`
			UPDATE posts SET upvotes = $2, downvotes = $3, comments_count = $4, status = $5,
				created_at = now() - make_interval(days => $6), moderation_pending = $7
			WHERE id = $1`, add.Result.ID, post.up, post.down, post.comments, post.status, post.ageDays, post.pending)
		Expect(err).IsNil()
		for _, tagID := range post.tags {
			_, err := trx.Execute("INSERT INTO post_tags (tag_id, post_id, created_at, created_by_id, tenant_id) VALUES ($1, $2, now(), $3, $4)",
				tagID, add.Result.ID, jonSnow.ID, demoTenant.ID)
			Expect(err).IsNil()
		}
		f.numbers[post.title] = add.Result.Number
		f.numberTitles[add.Result.Number] = post.title
	}

	// Seeded posts must not interfere with the fixture's expectations.
	_, err := trx.Execute("UPDATE posts SET status = $1 WHERE tenant_id = $2 AND title NOT LIKE 'Export fixture %'", enum.PostDeleted, demoTenant.ID)
	Expect(err).IsNil()
	return f
}

func (f exportFixture) run(t *testing.T, seed string, sections ...entity.FeedbackExportSection) [][]string {
	t.Helper()
	q := &query.SelectFeedbackExportRows{Recipe: entity.FeedbackExportRecipe{Sections: sections}, Seed: seed}
	Expect(bus.Dispatch(f.ctx, q)).IsNil()
	titles := make([][]string, len(q.Result))
	for i, rows := range q.Result {
		titles[i] = []string{}
		for _, row := range rows {
			titles[i] = append(titles[i], f.numberTitles[row.Number])
		}
	}
	return titles
}

func pick(mode entity.FeedbackExportMode, count, minComments int) entity.FeedbackExportPick {
	return entity.FeedbackExportPick{Mode: mode, Count: count, MinComments: minComments}
}

func TestFeedbackExport_FiltersAndOrders(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()
	f := newExportFixture(t, ctx)

	section := func(include, exclude []int, maxAgeDays int, picks ...entity.FeedbackExportPick) entity.FeedbackExportSection {
		return entity.FeedbackExportSection{Name: "S", IncludeTags: include, ExcludeTags: exclude, Statuses: f.openPlannedStarted, MaxAgeDays: maxAgeDays, Picks: picks}
	}

	Expect(f.run(t, "s", section([]int{f.bug}, nil, 0, pick(entity.FeedbackExportTopVoted, 5, 0)))).Equals([][]string{{"A", "F", "B"}})
	Expect(f.run(t, "s", section([]int{f.bug}, nil, 30, pick(entity.FeedbackExportTopVoted, 5, 0)))).Equals([][]string{{"A", "B"}})
	Expect(f.run(t, "s", section(nil, []int{f.bug, f.qol}, 0, pick(entity.FeedbackExportTopVoted, 5, 0)))).Equals([][]string{{"G", "D"}})
	Expect(f.run(t, "s", section([]int{f.qol, f.arena}, nil, 0, pick(entity.FeedbackExportTopVoted, 5, 0)))).Equals([][]string{{"C", "F", "D"}})
	Expect(f.run(t, "s", section(nil, nil, 0, pick(entity.FeedbackExportControversial, 2, 0)))).Equals([][]string{{"B", "D"}})
	Expect(f.run(t, "s", section(nil, nil, 0, pick(entity.FeedbackExportDiscussed, 3, 0)))).Equals([][]string{{"B", "F", "G"}})

	minVotes := 25
	withMinVotes := section(nil, nil, 0, pick(entity.FeedbackExportTopVoted, 10, 0))
	withMinVotes.MinVotes = &minVotes
	Expect(f.run(t, "s", withMinVotes)).Equals([][]string{{"A", "C", "F"}})

	completed := section(nil, nil, 0, pick(entity.FeedbackExportDiscussed, 1, 0))
	completed.Statuses = []enum.PostStatus{enum.PostCompleted}
	Expect(f.run(t, "s", completed)).Equals([][]string{{"E"}})
}

func TestFeedbackExport_PicksNeverRepeatPosts(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()
	f := newExportFixture(t, ctx)

	bugs := entity.FeedbackExportSection{Name: "Bugs", IncludeTags: []int{f.bug}, Statuses: f.openPlannedStarted,
		Picks: []entity.FeedbackExportPick{pick(entity.FeedbackExportTopVoted, 1, 0), pick(entity.FeedbackExportDiscussed, 2, 0)}}
	everything := entity.FeedbackExportSection{Name: "Rest", Statuses: f.openPlannedStarted,
		Picks: []entity.FeedbackExportPick{pick(entity.FeedbackExportTopVoted, 10, 0)}}

	Expect(f.run(t, "s", bugs, everything)).Equals([][]string{{"A", "B", "F"}, {"C", "G", "D"}})
}

func TestFeedbackExport_RandomPicksRepeatForTheSameSeed(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()
	f := newExportFixture(t, ctx)

	random := entity.FeedbackExportSection{Name: "Random", Statuses: f.openPlannedStarted,
		Picks: []entity.FeedbackExportPick{pick(entity.FeedbackExportRandom, 2, 10)}}

	first := f.run(t, "alpha", random)
	Expect(f.run(t, "alpha", random)).Equals(first)
	Expect(len(first[0])).Equals(2)
	for _, title := range first[0] {
		Expect(title == "B" || title == "F").IsTrue()
	}

	seen := map[string]bool{}
	for _, seed := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		random.Picks[0].Count = 1
		seen[f.run(t, seed, random)[0][0]] = true
	}
	Expect(len(seen)).Equals(2)
}

func TestFeedbackExport_RowsCarryTags(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()
	f := newExportFixture(t, ctx)

	q := &query.SelectFeedbackExportRows{Recipe: entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{{
		Name: "F", IncludeTags: []int{f.arena}, ExcludeTags: []int{f.qol}, Statuses: f.openPlannedStarted,
		Picks: []entity.FeedbackExportPick{pick(entity.FeedbackExportTopVoted, 1, 10)},
	}}}}
	Expect(bus.Dispatch(f.ctx, q)).IsNil()
	Expect(q.Result[0]).HasLen(1)
	row := q.Result[0][0]
	Expect(row.Number).Equals(f.numbers["F"])
	Expect(row.Votes).Equals(30)
	Expect(row.Comments).Equals(12)
	Expect(row.Pick).Equals(entity.FeedbackExportTopVoted)
	Expect(row.TagIDs).Equals([]int{f.bug, f.arena})
}

func TestFeedbackExportPresets_SaveListDelete(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	adminCtx := withTenant(withUser(ctx, jonSnow), demoTenant)
	recipe := entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{{
		Name:        "Top",
		Statuses:    []enum.PostStatus{enum.PostOpen},
		IncludeTags: []int{},
		ExcludeTags: []int{},
		Picks:       []entity.FeedbackExportPick{pick(entity.FeedbackExportTopVoted, 20, 0)},
	}}}

	weekly := &cmd.CreateFeedbackExportPreset{ID: "11111111111111111111111111111111", Name: "Weekly", Recipe: recipe}
	arena := &cmd.CreateFeedbackExportPreset{ID: "22222222222222222222222222222222", Name: "Arena", Recipe: recipe}
	Expect(bus.Dispatch(adminCtx, weekly, arena)).IsNil()

	renamed := &cmd.UpdateFeedbackExportPreset{
		SubmissionID: "rename",
		ID:           weekly.Result.ID, Name: "Weekly sheet", Recipe: recipe,
		Saved: entity.FeedbackExportPresetContent{Name: weekly.Name, Recipe: weekly.Recipe},
	}
	Expect(bus.Dispatch(adminCtx, renamed)).IsNil()

	list := &query.ListFeedbackExportPresets{}
	Expect(bus.Dispatch(adminCtx, list)).IsNil()
	Expect(list.Result).HasLen(2)
	Expect(list.Result[0].Name).Equals("Arena")
	Expect(list.Result[1].Name).Equals("Weekly sheet")
	Expect(list.Result[1].UpdatedBy).Equals(jonSnow.Name)
	Expect(list.Result[1].Recipe).Equals(recipe)

	otherTenant := withTenant(withUser(ctx, tonyStark), avengersTenant)
	other := &query.ListFeedbackExportPresets{}
	Expect(bus.Dispatch(otherTenant, other)).IsNil()
	Expect(other.Result).HasLen(0)
	Expect(bus.Dispatch(otherTenant, &cmd.DeleteFeedbackExportPreset{ID: arena.Result.ID})).Equals(app.ErrNotFound)

	Expect(bus.Dispatch(adminCtx, &cmd.DeleteFeedbackExportPreset{ID: arena.Result.ID})).IsNil()
	Expect(bus.Dispatch(adminCtx, list)).IsNil()
	Expect(list.Result).HasLen(1)
}

func TestFeedbackExportPrivateTagsRequireReadPermission(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()
	f := newExportFixture(t, ctx)

	if _, err := trx.Execute("UPDATE tags SET is_public = FALSE WHERE id = $1", f.bug); err != nil {
		t.Fatal(err)
	}
	viewer := *jonSnow
	viewer.Role = enum.RoleHelper
	demoTenant.RolePermissions = entity.RolePermissions{
		enum.RoleHelper: {
			entity.ExportFeedback:  true,
			entity.ViewPrivateTags: false,
		},
	}
	f.ctx = withTenant(withUser(ctx, &viewer), demoTenant)

	section := entity.FeedbackExportSection{
		Statuses: f.openPlannedStarted,
		Picks:    []entity.FeedbackExportPick{pick(entity.FeedbackExportTopVoted, 10, 0)},
	}
	q := &query.SelectFeedbackExportRows{Recipe: entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{section}}}
	if err := bus.Dispatch(f.ctx, q); err != nil {
		t.Fatal(err)
	}
	for _, row := range q.Result[0] {
		for _, tagID := range row.TagIDs {
			if tagID == f.bug {
				t.Fatal("export exposed private tag membership")
			}
		}
	}

	section.IncludeTags = []int{f.bug}
	Expect(f.run(t, "s", section)).Equals([][]string{{}})
	section.IncludeTags = nil
	section.ExcludeTags = []int{f.bug}
	Expect(f.run(t, "s", section)).Equals([][]string{{"A", "C", "F", "G", "B", "D"}})

	demoTenant.RolePermissions[enum.RoleHelper][entity.ViewPrivateTags] = true
	Expect(f.run(t, "s", section)).Equals([][]string{{"C", "G", "D"}})
	section.ExcludeTags = nil
	section.IncludeTags = []int{f.bug}
	Expect(f.run(t, "s", section)).Equals([][]string{{"A", "F", "B"}})

	section.Picks = []entity.FeedbackExportPick{
		pick(entity.FeedbackExportRandom, 10, 0),
		pick(entity.FeedbackExportRandom, 10, 1),
		pick(entity.FeedbackExportRandom, 10, 2),
	}
	for _, allowed := range []bool{false, true} {
		demoTenant.RolePermissions[enum.RoleHelper][entity.ViewPrivateTags] = allowed
		section.IncludeTags = []int{f.bug}
		section.ExcludeTags = nil
		included := f.run(t, "shared-rank", section)
		section.IncludeTags = nil
		section.ExcludeTags = []int{f.bug}
		excluded := f.run(t, "shared-rank", section)

		if allowed {
			Expect(included[0]).HasLen(3)
			Expect(excluded[0]).HasLen(3)
		} else {
			Expect(included[0]).HasLen(0)
			Expect(excluded[0]).HasLen(6)
		}
	}
}
