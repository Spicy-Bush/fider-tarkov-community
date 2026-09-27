package cmd

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type SetModerationPending struct {
	ContentType string
	ContentID   int
	Pending     bool

	Comment    *entity.Comment
	Discussion *entity.Discussion
}

type ScheduleModeration struct {
	ContentType string
	ContentID   int
}

type ModerationCheck struct {
	ProviderAvailableAt time.Time `json:"-"`
	Slot                int       `json:"-"`
	SlotClaim           int64     `json:"-"`
	TenantID            int       `json:"-"`
	ContentType         string    `json:"contentType"`
	ContentID           int       `json:"contentID"`
	Revision            int64     `json:"revision"`
	Claim               int64     `json:"-"`
	Text                string    `json:"-"`
	BlobKeys            []string  `json:"-"`
	Attempts            int       `json:"attempts"`
	State               string    `json:"state"`
	LastError           string    `json:"lastError"`
}

type ClaimModeration struct{ Result *ModerationCheck }

type ModerationOutcome uint8

const (
	ModerationReviewed ModerationOutcome = iota + 1
	ModerationRetry
	ModerationFailed
)

type ModerationFinding struct {
	Category string
	Score    float64
}

type FinishModeration struct {
	Check             ModerationCheck
	Outcome           ModerationOutcome
	Findings          []ModerationFinding
	RetryAfterSeconds int
	CooldownSeconds   int
	Error             string
	Result            string
	Applied           bool
	PublishedName     string
}

type ListModerationFailures struct {
	Result []ModerationCheck
	Total  int
	Failed int
}

type RetryModerationFailures struct{ Count int64 }

type SaveProfileName struct {
	UserID  int
	Name    string
	Review  bool
	Pending bool
}

type SaveProfileAvatar struct {
	UserID     int
	BlobKey    string
	AvatarType enum.AvatarType
	Review     bool
	Pending    bool
}

type GetProfileModeration struct {
	UserID int
	Result []ModerationCheck
}
