package cmd

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

type CreateFeedbackExportPreset struct {
	ID     string
	Name   string
	Recipe entity.FeedbackExportRecipe

	Result *entity.FeedbackExportPreset
}

type UpdateFeedbackExportPreset struct {
	SubmissionID string
	ID           string
	Name         string
	Recipe       entity.FeedbackExportRecipe
	Saved        entity.FeedbackExportPresetContent

	Result *entity.FeedbackExportPresetUpdate
}

type DeleteFeedbackExportPreset struct {
	ID string
}
