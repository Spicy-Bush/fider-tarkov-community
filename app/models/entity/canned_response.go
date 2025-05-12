package entity

import "time"

// CannedResponse represents a predefined response template
type CannedResponse struct {
	ID          int       `json:"id" db:"id"`
	Type        string    `json:"type" db:"type"`
	Title       string    `json:"title" db:"title"`
	Content     string    `json:"content" db:"content"`
	Duration    string    `json:"duration,omitempty" db:"duration"`
	IsActive    bool      `json:"isActive" db:"is_active"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
	CreatedBy   *User     `json:"-"`
	CreatedByID *int      `json:"createdBy,omitempty" db:"created_by_id"`
}
