package entity

import "time"

// CatalogPlacementIDs is the static public allow-list (SSR/select). Adding a
// placement is a catalog row + frame spec + optional adsense + a page insertion
// point; adsselect is not retouched.
var CatalogPlacementIDs = map[string]struct{}{
	"feed_native":      {},
	"sidebar_top":      {},
	"post_below_title": {},
	"pages_header":     {},
}

func IsCatalogPlacementID(id string) bool {
	_, ok := CatalogPlacementIDs[id]
	return ok
}

// AdPlacement is a global (non-tenant) catalog row for a renderable ad slot.
type AdPlacement struct {
	ID            string `json:"id" db:"id"`
	Name          string `json:"name" db:"name"`
	Description   string `json:"description" db:"description"`
	Kind          string `json:"kind" db:"kind"`
	MaxWidth      *int   `json:"maxWidth,omitempty" db:"max_width"`
	MaxHeight     *int   `json:"maxHeight,omitempty" db:"max_height"`
	Sort          int    `json:"sort" db:"sort"`
	Enabled       bool   `json:"enabled" db:"enabled"`
	AdSenseSlotID string `json:"adsenseSlotId" db:"adsense_slot_id"`
	AdSenseFormat string `json:"adsenseFormat" db:"adsense_format"`
	EmptyPolicy   string `json:"emptyPolicy" db:"empty_policy"` // collapse | reserve
}

// CreativeVersion is an immutable creative payload owned by a campaign.
type CreativeVersion struct {
	ID         int       `json:"id" db:"id"`
	CampaignID int       `json:"campaignId" db:"campaign_id"`
	VersionNo  int       `json:"versionNo" db:"version_no"`
	ImageURL   string    `json:"imageUrl" db:"image_url"`
	HTML       string    `json:"html" db:"html"`
	ClickURL   string    `json:"clickUrl" db:"click_url"`
	CreatedAt  time.Time `json:"createdAt" db:"created_at"`
}

// CampaignAssignment binds a placement to a creative version under a campaign.
type CampaignAssignment struct {
	ID                int    `json:"id" db:"id"`
	CampaignID        int    `json:"campaignId" db:"campaign_id"`
	PlacementID       string `json:"placementId" db:"placement_id"`
	CreativeVersionID int    `json:"creativeVersionId" db:"creative_version_id"`
}

// PublicAd is the safe public payload returned by POST /api/v1/ads/select.
// Advertiser must be non-blank; callers omit/null the instance when empty (#39).
type PublicAd struct {
	CampaignID        int    `json:"campaignId"`
	Advertiser        string `json:"advertiser"`
	PlacementID       string `json:"placementId"`
	CreativeVersionID int    `json:"creativeVersionId"`
	ImageURL          string `json:"imageUrl"`
	HTML              string `json:"html"`
	ClickPath         string `json:"clickPath"`
}
