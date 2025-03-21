package entity

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

// VoteUser represents a user that voted on a post
type VoteUser struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email,omitempty"`
	AvatarURL string `json:"avatarURL,omitempty"`
}

// Vote represents a vote given by a user on a post
type Vote struct {
	User      *VoteUser `json:"user"`
	CreatedAt time.Time `json:"createdAt"`
	VoteType  enum.VoteType
}
