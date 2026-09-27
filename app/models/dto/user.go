package dto

import "time"

type UserNames struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type UserStandingEntry struct {
	ID        int        `json:"id"`
	Reason    string     `json:"reason"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	IsActive  bool       `json:"isActive"`
}
