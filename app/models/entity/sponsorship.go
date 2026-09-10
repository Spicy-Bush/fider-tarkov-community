package entity

import (
	"strings"
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

// SponsorshipCampaign is a scheduled house ad for one or more slots.
// Name is internal (billing/reference). Advertiser is the public company label for disclosure.
// Slots is a comma-separated list (e.g. "feed_native,sidebar_top").
// CreativeImageURLs maps slot_id → image URL (optional per placement).
// CreativeImageURL is the legacy single-image fallback when a slot key is missing.
type SponsorshipCampaign struct {
	ID                int               `json:"id" db:"id"`
	Name              string            `json:"name" db:"name"`
	Advertiser        string            `json:"advertiser" db:"advertiser"`
	Slots             string            `json:"slots" db:"slots"`
	CreativeImageURL  string            `json:"creativeImageUrl" db:"creative_image_url"`
	CreativeImageURLs map[string]string `json:"creativeImageUrls,omitempty"`
	CreativeHTML      string            `json:"creativeHtml" db:"creative_html"`
	ClickURL          string            `json:"clickUrl" db:"click_url"`
	StartAt           time.Time         `json:"startAt" db:"start_at"`
	EndAt             time.Time         `json:"endAt" db:"end_at"`
	Weight            int               `json:"weight" db:"weight"`
	Locale            string            `json:"locale" db:"locale"`
	Enabled           bool              `json:"enabled" db:"enabled"`
	Clicks            int               `json:"clicks" db:"clicks"`
	PackageID         *int              `json:"packageId,omitempty" db:"package_id"`
	CreatedAt         time.Time         `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time         `json:"updatedAt" db:"updated_at"`
}

// ImageURLForSlot returns the creative image for slotID: map entry first, then legacy column.
func (c *SponsorshipCampaign) ImageURLForSlot(slotID string) string {
	if c == nil {
		return ""
	}
	slotID = strings.TrimSpace(slotID)
	if c.CreativeImageURLs != nil {
		if u := strings.TrimSpace(c.CreativeImageURLs[slotID]); u != "" {
			return u
		}
	}
	return strings.TrimSpace(c.CreativeImageURL)
}

// PublicSponsorshipCampaign is the safe public payload for rendering a slot.
// CreativeImageURL is already resolved for SlotID (per-slot map or legacy fallback).
type PublicSponsorshipCampaign struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Advertiser       string `json:"advertiser"`
	SlotID           string `json:"slotId"`
	CreativeImageURL string `json:"creativeImageUrl,omitempty"`
	CreativeHTML     string `json:"creativeHtml,omitempty"`
	ClickPath        string `json:"clickPath"`
}
