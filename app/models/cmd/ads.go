package cmd

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

// CreateCreativeVersion appends an immutable creative version (next version_no).
// ConfigVersion is required OCC token; store rejects mismatch and bumps campaign version.
// NewConfigVersion is the persisted token after a successful bump (server is OCC source).
type CreateCreativeVersion struct {
	CampaignID       int
	ImageURL         string
	HTML             string
	ClickURL         string
	ConfigVersion    int
	Result           *entity.CreativeVersion
	NewConfigVersion int
}

// CampaignAssignmentInput is one desired placement binding for a graph save.
type CampaignAssignmentInput struct {
	PlacementID       string
	CreativeVersionID int
}

// SaveSponsorshipCampaignGraph updates slim campaign fields and replaces the
// assignment set in one transaction under OCC (config_version).
type SaveSponsorshipCampaignGraph struct {
	ID                int
	Name              string
	Advertiser        string
	StartAt           time.Time
	EndAt             time.Time
	Weight            int
	Locale            string
	Enabled           bool
	PackageID         *int
	ConfigVersion     int
	Assignments       []CampaignAssignmentInput
	Result            *entity.SponsorshipCampaign
	AssignmentResults []*entity.CampaignAssignment
}
