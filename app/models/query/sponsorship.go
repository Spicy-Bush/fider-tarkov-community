package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

type ListSponsorshipPackages struct {
	Result []*entity.SponsorshipPackage
}

type GetSponsorshipPackageByID struct {
	ID     int
	Result *entity.SponsorshipPackage
}

type ListSponsorshipCampaigns struct {
	Result []*entity.SponsorshipCampaign
}

type GetSponsorshipCampaignByID struct {
	ID     int
	Result *entity.SponsorshipCampaign
}

// GetActiveSponsorshipForSlot picks one active house campaign for a slot.
type GetActiveSponsorshipForSlot struct {
	SlotID string
	Locale string
	Result *entity.SponsorshipCampaign
}

// GetActiveSponsorshipForSlots picks at most one campaign per requested slot (one DB read).
type GetActiveSponsorshipForSlots struct {
	SlotIDs []string
	Locale  string
	Result  map[string]*entity.SponsorshipCampaign
}
