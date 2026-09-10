package cmd

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

type CreateSponsorshipPackage struct {
	Slug         string
	Name         string
	Description  string
	Slots        string
	DurationDays int
	Sort         int
	Result       *entity.SponsorshipPackage
}

type UpdateSponsorshipPackage struct {
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

// CreateCampaignVersionInput is an optional first creative on POST /campaigns.
type CreateCampaignVersionInput struct {
	ImageURL string
	HTML     string
	ClickURL string
}

type CreateSponsorshipCampaign struct {
	Name              string
	Advertiser        string
	StartAt           time.Time
	EndAt             time.Time
	Weight            int
	Locale            string
	Enabled           bool
	PackageID         *int
	Version           *CreateCampaignVersionInput // optional first version
	Assignments       []CampaignAssignmentInput   // optional; CreativeVersionID 0 binds to Version
	Result            *entity.SponsorshipCampaign
	VersionResult     *entity.CreativeVersion
	AssignmentResults []*entity.CampaignAssignment
}

type UpdateSponsorshipCampaign struct {
	ID            int
	Name          string
	Advertiser    string
	StartAt       time.Time
	EndAt         time.Time
	Weight        int
	Locale        string
	Enabled       bool
	PackageID     *int
	ConfigVersion int // required OCC token; store rejects mismatch and bumps
	Result        *entity.SponsorshipCampaign
}

type DeleteSponsorshipCampaign struct {
	ID int
}

type IncrementSponsorshipClick struct {
	ID int
}
