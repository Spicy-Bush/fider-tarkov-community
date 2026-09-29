package actions_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/actions"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
)

func exportSection(picks ...entity.FeedbackExportPick) entity.FeedbackExportSection {
	return entity.FeedbackExportSection{Name: "Top", Statuses: []enum.PostStatus{enum.PostOpen}, Picks: picks}
}

func TestPreviewFeedbackExport_Validate(t *testing.T) {
	RegisterT(t)

	top := entity.FeedbackExportPick{Mode: entity.FeedbackExportTopVoted, Count: 20}
	for _, test := range []struct {
		name   string
		recipe entity.FeedbackExportRecipe
		field  string
	}{
		{"valid", entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{exportSection(top)}}, ""},
		{"no sections", entity.FeedbackExportRecipe{}, "recipe"},
		{"no picks", entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{exportSection()}}, "recipe.sections.0.picks"},
		{"no statuses", entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{{Picks: []entity.FeedbackExportPick{top}}}}, "recipe.sections.0.statuses"},
		{"deleted posts", entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{{Statuses: []enum.PostStatus{enum.PostDeleted}, Picks: []entity.FeedbackExportPick{top}}}}, "recipe.sections.0.statuses"},
		{"archived posts", entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{{Statuses: []enum.PostStatus{enum.PostArchived}, Picks: []entity.FeedbackExportPick{top}}}}, "recipe.sections.0.statuses"},
		{"unknown mode", entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{exportSection(entity.FeedbackExportPick{Mode: "newest", Count: 5})}}, "recipe.sections.0.picks.0.mode"},
		{"empty pick", entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{exportSection(entity.FeedbackExportPick{Mode: entity.FeedbackExportTopVoted})}}, "recipe.sections.0.picks.0.count"},
		{"pick too large", entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{exportSection(entity.FeedbackExportPick{Mode: entity.FeedbackExportTopVoted, Count: 201})}}, "recipe.sections.0.picks.0.count"},
		{"too many rows", entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{
			exportSection(entity.FeedbackExportPick{Mode: entity.FeedbackExportTopVoted, Count: 200}, entity.FeedbackExportPick{Mode: entity.FeedbackExportRandom, Count: 200}),
			exportSection(entity.FeedbackExportPick{Mode: entity.FeedbackExportTopVoted, Count: 101}),
		}}, "recipe"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := (&actions.PreviewFeedbackExport{Recipe: test.recipe}).Validate(context.Background(), nil)
			if test.field == "" {
				ExpectSuccess(result)
			} else {
				ExpectFailed(result, test.field)
			}
		})
	}

	ExpectFailed((&actions.PreviewFeedbackExport{Recipe: entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{exportSection(top)}}, Seed: strings.Repeat("x", 65)}).Validate(context.Background(), nil), "seed")
}

func TestCreateFeedbackExportPreset_Validate(t *testing.T) {
	RegisterT(t)

	recipe := entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{exportSection(entity.FeedbackExportPick{Mode: entity.FeedbackExportTopVoted, Count: 10})}}
	const id = "0123456789abcdef0123456789abcdef"

	ExpectSuccess((&actions.CreateFeedbackExportPreset{ID: id, Name: " Bugs ", Recipe: recipe}).Validate(context.Background(), nil))
	ExpectSuccess((&actions.CreateFeedbackExportPreset{ID: id, Name: strings.Repeat("日", 60), Recipe: recipe}).Validate(context.Background(), nil))
	ExpectFailed((&actions.CreateFeedbackExportPreset{ID: id, Name: strings.Repeat("日", 61), Recipe: recipe}).Validate(context.Background(), nil), "name")
	ExpectFailed((&actions.CreateFeedbackExportPreset{ID: id, Name: "  ", Recipe: recipe}).Validate(context.Background(), nil), "name")
	ExpectFailed((&actions.CreateFeedbackExportPreset{Name: "Weekly", Recipe: recipe}).Validate(context.Background(), nil), "id")

	update := &actions.UpdateFeedbackExportPreset{SubmissionID: "preset-save", ID: id, Name: "Weekly", Recipe: recipe}
	ExpectFailed(update.Validate(context.Background(), nil), "saved")
	update.Saved = &entity.FeedbackExportPresetContent{Name: "Original", Recipe: recipe}
	ExpectSuccess(update.Validate(context.Background(), nil))

	for _, identity := range []string{"", strings.Repeat("x", 129)} {
		update.SubmissionID = identity
		ExpectFailed(update.Validate(context.Background(), nil), "submissionId")
	}

	for _, malformed := range []string{"", "0", "1", "not-an-id", strings.Repeat("f", 31), strings.Repeat("F", 32)} {
		update := (&actions.UpdateFeedbackExportPreset{ID: malformed, Name: "Weekly", Recipe: recipe}).Validate(context.Background(), nil)
		Expect(update.Err).Equals(app.ErrNotFound)
	}
}

func TestFeedbackExportRecipeCanonicalInputs(t *testing.T) {
	RegisterT(t)
	section := exportSection(entity.FeedbackExportPick{Mode: entity.FeedbackExportTopVoted, Count: 10})
	section.Name = strings.Repeat("日", 60)
	section.Statuses = []enum.PostStatus{enum.PostPlanned, enum.PostOpen, enum.PostPlanned}
	section.IncludeTags = []int{3, 1, 3}
	section.ExcludeTags = []int{5, 4, 5}
	action := &actions.PreviewFeedbackExport{Recipe: entity.FeedbackExportRecipe{Sections: []entity.FeedbackExportSection{section}}}

	ExpectSuccess(action.Validate(context.Background(), nil))
	Expect(action.Recipe.Sections[0].Statuses).Equals([]enum.PostStatus{enum.PostOpen, enum.PostPlanned})
	Expect(action.Recipe.Sections[0].IncludeTags).Equals([]int{1, 3})
	Expect(action.Recipe.Sections[0].ExcludeTags).Equals([]int{4, 5})

	action.Recipe.Sections[0].Statuses = make([]enum.PostStatus, 100000)
	ExpectFailed(action.Validate(context.Background(), nil), "recipe.sections.0.statuses")

	action.Recipe.Sections[0].Statuses = []enum.PostStatus{enum.PostOpen}
	action.Recipe.Sections[0].Name = strings.Repeat("日", 61)
	ExpectFailed(action.Validate(context.Background(), nil), "recipe.sections.0.name")
}

func TestFeedbackExportRejectsDatabaseInvalidInputs(t *testing.T) {
	RegisterT(t)
	for _, change := range []struct {
		name   string
		mutate func(*actions.PreviewFeedbackExport)
		field  string
	}{
		{"seed NUL", func(a *actions.PreviewFeedbackExport) { a.Seed = "a\x00b" }, "seed"},
		{"section NUL", func(a *actions.PreviewFeedbackExport) { a.Recipe.Sections[0].Name = "a\x00b" }, "recipe.sections.0.name"},
		{"tag overflow", func(a *actions.PreviewFeedbackExport) { a.Recipe.Sections[0].IncludeTags = []int{1 << 32} }, "recipe.sections.0.tags"},
		{"comments overflow", func(a *actions.PreviewFeedbackExport) { a.Recipe.Sections[0].Picks[0].MinComments = 1 << 32 }, "recipe.sections.0.picks.0.minComments"},
	} {
		t.Run(change.name, func(t *testing.T) {
			action := &actions.PreviewFeedbackExport{Recipe: entity.FeedbackExportRecipe{
				Sections: []entity.FeedbackExportSection{exportSection(entity.FeedbackExportPick{
					Mode: entity.FeedbackExportTopVoted, Count: 10,
				})},
			}}
			change.mutate(action)

			ExpectFailed(action.Validate(context.Background(), nil), change.field)
		})
	}
}
