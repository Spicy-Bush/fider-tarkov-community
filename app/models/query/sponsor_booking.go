package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

type SponsorManagement struct {
	Browse      SponsorBrowse              `json:"browse"`
	NextPage    bool                       `json:"nextPage"`
	NextArtwork bool                       `json:"nextArtwork"`
	Campaigns   []*entity.SponsorCampaign  `json:"campaigns"`
	Creatives   []*entity.SponsorCreative  `json:"creatives"`
	Placements  []entity.SponsorPlacement  `json:"placements"`
	Exclusions  []*entity.SponsorExclusion `json:"exclusions"`
}

type GetSponsorManagement struct {
	Browse SponsorBrowse
	Result SponsorManagement
}

type SponsorBrowse struct {
	Page        int    `json:"page"`
	CampaignID  int    `json:"campaignId"`
	ArtworkPage int    `json:"artworkPage"`
	Search      string `json:"search"`
}

type GetSponsorPlacements struct {
	Result []entity.SponsorPlacement
}

type GetSponsorReport struct {
	CampaignID  int
	Allocations []*entity.SponsorAllocation `json:"allocations"`
	Changes     []*entity.SponsorChange     `json:"changes"`
}
