package entity

import (
	"time"
)

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

// SponsorshipCampaign is a slim schedule/weight umbrella.
// Name is internal (billing/reference). Advertiser is the public company label for disclosure.
// Creatives live in creative_versions; placement bindings in campaign_assignments.
// ConfigVersion is OCC token bumped on successful admin writes.
type SponsorshipCampaign struct {
	ID            int       `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	Advertiser    string    `json:"advertiser" db:"advertiser"`
	StartAt       time.Time `json:"startAt" db:"start_at"`
	EndAt         time.Time `json:"endAt" db:"end_at"`
	Weight        int       `json:"weight" db:"weight"`
	Locale        string    `json:"locale" db:"locale"`
	Enabled       bool      `json:"enabled" db:"enabled"`
	Clicks        int       `json:"clicks" db:"clicks"`
	PackageID     *int      `json:"packageId,omitempty" db:"package_id"`
	ConfigVersion int       `json:"configVersion" db:"config_version"`
	CreatedAt     time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt     time.Time `json:"updatedAt" db:"updated_at"`
}
