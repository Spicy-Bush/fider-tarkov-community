package entity

import "time"

// SponsorshipPackage is a sellable package definition (no prices stored).
type SponsorshipPackage struct {
	ID           int       `json:"id" db:"id"`
	Slug         string    `json:"slug" db:"slug"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	Slots        string    `json:"slots" db:"slots"`
	DurationDays int       `json:"durationDays" db:"duration_days"`
	Sort         int       `json:"sort" db:"sort"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
}

// SponsorshipCampaign is a scheduled house ad for a slot.
type SponsorshipCampaign struct {
	ID               int       `json:"id" db:"id"`
	Name             string    `json:"name" db:"name"`
	SlotID           string    `json:"slotId" db:"slot_id"`
	CreativeImageURL string    `json:"creativeImageUrl" db:"creative_image_url"`
	CreativeHTML     string    `json:"creativeHtml" db:"creative_html"`
	ClickURL         string    `json:"clickUrl" db:"click_url"`
	StartAt          time.Time `json:"startAt" db:"start_at"`
	EndAt            time.Time `json:"endAt" db:"end_at"`
	Weight           int       `json:"weight" db:"weight"`
	Locale           string    `json:"locale" db:"locale"`
	Enabled          bool      `json:"enabled" db:"enabled"`
	Clicks           int       `json:"clicks" db:"clicks"`
	PackageID        *int      `json:"packageId,omitempty" db:"package_id"`
	CreatedAt        time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt        time.Time `json:"updatedAt" db:"updated_at"`
}

// PublicSponsorshipCampaign is the safe public payload for rendering a slot.
type PublicSponsorshipCampaign struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	SlotID           string `json:"slotId"`
	CreativeImageURL string `json:"creativeImageUrl,omitempty"`
	CreativeHTML     string `json:"creativeHtml,omitempty"`
	ClickPath        string `json:"clickPath"`
}
