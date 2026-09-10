package cmd

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

// CreateCreativeVersion appends an immutable creative version (next version_no).
type CreateCreativeVersion struct {
	CampaignID int
	ImageURL   string
	HTML       string
	ClickURL   string
	Result     *entity.CreativeVersion
}

// UpsertCampaignAssignment sets the creative version for a campaign×placement binding.
type UpsertCampaignAssignment struct {
	CampaignID        int
	PlacementID       string
	CreativeVersionID int
	Result            *entity.CampaignAssignment
}
