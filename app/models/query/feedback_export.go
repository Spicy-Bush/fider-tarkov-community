package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

type ListFeedbackExportPresets struct {
	Result []*entity.FeedbackExportPreset
}

// Result has one list per recipe section. Seed makes random picks repeatable.
type SelectFeedbackExportRows struct {
	Recipe entity.FeedbackExportRecipe
	Seed   string

	Result [][]*entity.FeedbackExportRow
}
