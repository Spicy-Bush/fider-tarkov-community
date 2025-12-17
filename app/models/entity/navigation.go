package entity

import "time"

type NavigationLink struct {
	ID           int       `json:"id" db:"id"`
	Title        string    `json:"title" db:"title"`
	URL          string    `json:"url" db:"url"`
	DisplayOrder int       `json:"displayOrder" db:"display_order"`
	Location     string    `json:"location" db:"location"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt    time.Time `json:"updatedAt" db:"updated_at"`
}
