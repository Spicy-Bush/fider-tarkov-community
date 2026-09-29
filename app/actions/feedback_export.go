package actions

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type PreviewFeedbackExport struct {
	Recipe entity.FeedbackExportRecipe `json:"recipe"`
	Seed   string                      `json:"seed"`
}

func (action *PreviewFeedbackExport) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ExportFeedback)
}

func (action *PreviewFeedbackExport) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if utf8.RuneCountInString(action.Seed) > 64 {
		result.AddFieldFailure("seed", "Seed must have at most 64 characters.")
	}
	if strings.ContainsRune(action.Seed, 0) {
		result.AddFieldFailure("seed", "Seed contains an invalid character.")
	}

	validateFeedbackExportRecipe(result, &action.Recipe)
	return result
}

type CreateFeedbackExportPreset struct {
	ID     string                      `json:"id"`
	Name   string                      `json:"name"`
	Recipe entity.FeedbackExportRecipe `json:"recipe"`
}

func (action *CreateFeedbackExportPreset) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ExportFeedback)
}

func (action *CreateFeedbackExportPreset) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if !validFeedbackExportID(action.ID) {
		result.AddFieldFailure("id", "Choose a valid preset ID.")
	}

	validateFeedbackExportPreset(result, &action.Name, &action.Recipe)
	return result
}

type UpdateFeedbackExportPreset struct {
	SubmissionID string                              `json:"submissionId"`
	ID           string                              `json:"-" route:"id"`
	Name         string                              `json:"name"`
	Recipe       entity.FeedbackExportRecipe         `json:"recipe"`
	Saved        *entity.FeedbackExportPresetContent `json:"saved"`
}

func (action *UpdateFeedbackExportPreset) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ExportFeedback)
}

func (action *UpdateFeedbackExportPreset) Validate(ctx context.Context, user *entity.User) *validate.Result {
	if !validFeedbackExportID(action.ID) {
		return validate.Error(app.ErrNotFound)
	}

	result := validate.Success()
	validateFeedbackExportPreset(result, &action.Name, &action.Recipe)
	if action.SubmissionID == "" || len(action.SubmissionID) > 128 {
		result.AddFieldFailure("submissionId", "A valid submission identity is required.")
	}

	if action.Saved == nil {
		result.AddFieldFailure("saved", "The saved preset is required.")
	} else {
		validateFeedbackExportPreset(result, &action.Saved.Name, &action.Saved.Recipe)
	}

	return result
}

type DeleteFeedbackExportPreset struct {
	ID string `json:"-" route:"id"`
}

func (action *DeleteFeedbackExportPreset) IsAuthorized(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ExportFeedback)
}

func (action *DeleteFeedbackExportPreset) Validate(ctx context.Context, user *entity.User) *validate.Result {
	if !validFeedbackExportID(action.ID) {
		return validate.Error(app.ErrNotFound)
	}

	return validate.Success()
}

func validFeedbackExportID(id string) bool {
	if len(id) != 32 {
		return false
	}

	for _, character := range id {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validateFeedbackExportPreset(result *validate.Result, name *string, recipe *entity.FeedbackExportRecipe) {
	*name = strings.TrimSpace(*name)
	if *name == "" {
		result.AddFieldFailure("name", "Name is required.")
	} else if utf8.RuneCountInString(*name) > 60 {
		result.AddFieldFailure("name", "Name must have at most 60 characters.")
	}
	if strings.ContainsRune(*name, 0) {
		result.AddFieldFailure("name", "Name contains an invalid character.")
	}

	validateFeedbackExportRecipe(result, recipe)
}

func validateFeedbackExportRecipe(result *validate.Result, recipe *entity.FeedbackExportRecipe) {
	if len(recipe.Sections) == 0 || len(recipe.Sections) > entity.MaxFeedbackExportSections {
		result.AddFieldFailure("recipe", fmt.Sprintf("Add between 1 and %d sections.", entity.MaxFeedbackExportSections))
		return
	}

	rows := 0
	for i := range recipe.Sections {
		section := &recipe.Sections[i]
		field := fmt.Sprintf("recipe.sections.%d", i)
		if utf8.RuneCountInString(section.Name) > 60 {
			result.AddFieldFailure(field+".name", "Section names must have at most 60 characters.")
		}
		if strings.ContainsRune(section.Name, 0) {
			result.AddFieldFailure(field+".name", "Section name contains an invalid character.")
		}

		if len(section.Statuses) == 0 || len(section.Statuses) > len(entity.FeedbackExportStatuses) {
			result.AddFieldFailure(field+".statuses", "Choose between 1 and 6 statuses.")
		} else {
			for _, status := range section.Statuses {
				if !slices.Contains(entity.FeedbackExportStatuses, status) {
					result.AddFieldFailure(field+".statuses", fmt.Sprintf("Posts that are %s cannot be exported.", status.Name()))
				}
			}
		}

		for _, tags := range []*[]int{&section.IncludeTags, &section.ExcludeTags} {
			if len(*tags) > 100 {
				result.AddFieldFailure(field+".tags", "Too many tags.")
				continue
			}

			for _, id := range *tags {
				if id < 1 || id > math.MaxInt32 {
					result.AddFieldFailure(field+".tags", "Choose valid tags.")
					break
				}
			}
		}

		if section.MaxAgeDays < 0 || section.MaxAgeDays > 3650 {
			result.AddFieldFailure(field+".maxAgeDays", "Age must be between 0 and 3650 days.")
		}
		if section.MinVotes != nil && (*section.MinVotes < math.MinInt32 || *section.MinVotes > math.MaxInt32) {
			result.AddFieldFailure(field+".minVotes", "Minimum votes is out of range.")
		}

		if len(section.Picks) == 0 || len(section.Picks) > entity.MaxFeedbackExportPicks {
			result.AddFieldFailure(field+".picks", fmt.Sprintf("Add between 1 and %d picks.", entity.MaxFeedbackExportPicks))
			continue
		}

		for j, pick := range section.Picks {
			pickField := fmt.Sprintf("%s.picks.%d", field, j)
			if !pick.Mode.IsValid() {
				result.AddFieldFailure(pickField+".mode", "Unknown pick.")
			}
			if pick.Count < 1 || pick.Count > entity.MaxFeedbackExportPickSize {
				result.AddFieldFailure(pickField+".count", fmt.Sprintf("Take between 1 and %d posts.", entity.MaxFeedbackExportPickSize))
			} else {
				rows += pick.Count
			}
			if pick.MinComments < 0 || pick.MinComments > math.MaxInt32 {
				result.AddFieldFailure(pickField+".minComments", "Minimum comments must be between 0 and 2147483647.")
			}
		}
	}

	if rows > entity.MaxFeedbackExportRows {
		result.AddFieldFailure("recipe", fmt.Sprintf("An export can have at most %d posts.", entity.MaxFeedbackExportRows))
	}

	if result.Ok {
		recipe.CanonicalizeFilters()
	}
}
