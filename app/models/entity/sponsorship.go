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
