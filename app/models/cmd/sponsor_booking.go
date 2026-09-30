package cmd

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
)

type UploadSponsorImage struct {
	Name         string
	Content      []byte
	SubmissionID string
	Result       *dto.FileInfo
}

type SaveSponsorCampaign struct {
	BaseCampaign   *entity.SponsorCampaign
	BaseCreative   *entity.SponsorCreative
	Conflicts      entity.SponsorConflicts
	Problems       []string
	DraftCampaign  *entity.SponsorCampaign
	DraftCreative  *entity.SponsorCreative
	Campaign       entity.SponsorCampaign
	Creative       *entity.SponsorCreative
	SubmissionID   string
	Result         *entity.SponsorCampaign
	CreativeResult *entity.SponsorCreative
}

type SaveSponsorPlacement struct {
	SubmissionID string
	Placement    entity.SponsorPlacement
	Result       entity.SponsorPlacement
}

type DeleteSponsorCampaign struct {
	ID int
}

type DeleteSponsorCreative struct {
	ID int
}

type AllocateSponsors struct {
	PageID        string
	ExpiresAt     time.Time
	Context       entity.SponsorContext
	Opportunities []entity.SponsorOpportunity
	Result        map[string]entity.SponsorSelection
}

type PurgeSponsorOpportunities struct{}

type SaveSponsorExclusion struct {
	SubmissionID string
	Result       []*entity.SponsorExclusion
	Exclusion    entity.SponsorExclusion
	Excluded     bool
}

type RecordSponsorClick struct {
	Click adsselect.Click
}
