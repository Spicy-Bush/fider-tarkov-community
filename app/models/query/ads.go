package query

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/adsselect"
)

// ListAdPlacements returns the global placement catalog (enabled and disabled).
type ListAdPlacements struct {
	Result []*entity.AdPlacement
}

// GetActiveAdCandidates loads schedule/locale-filtered assignment candidates.
// Now must be injected by the handler (UTC).
type GetActiveAdCandidates struct {
	PlacementIDs []string
	Locale       string
	Now          time.Time
	Result       []adsselect.Candidate
}

// GetCreativeVersionsByIDs batch-loads creative version metadata by id.
type GetCreativeVersionsByIDs struct {
	IDs    []int
	Result map[int]*entity.CreativeVersion
}

// ListCreativeVersionsByCampaign returns all versions for a campaign (newest first).
type ListCreativeVersionsByCampaign struct {
	CampaignID int
	Result     []*entity.CreativeVersion
}

// ListCampaignAssignmentsByCampaign returns placement bindings for a campaign.
type ListCampaignAssignmentsByCampaign struct {
	CampaignID int
	Result     []*entity.CampaignAssignment
}
