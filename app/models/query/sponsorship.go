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
