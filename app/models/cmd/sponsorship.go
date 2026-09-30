package cmd

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

type CreateSponsorshipPackage struct {
	SubmissionID string `json:"submissionId"`
	Slug         string
	Name         string
	Description  string
	Slots        string
	DurationDays int
	Sort         int
	Result       *entity.SponsorshipPackage
}

type UpdateSponsorshipPackage struct {
	SubmissionID string `json:"submissionId"`
	ID           int
	Slug         string
	Name         string
	Description  string
	Slots        string
	DurationDays int
	Sort         int
	Result       *entity.SponsorshipPackage
}

type DeleteSponsorshipPackage struct {
	ID int
}
