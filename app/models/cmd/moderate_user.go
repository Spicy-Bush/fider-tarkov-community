package cmd

import (
	"time"
)

// MuteUser represents the command to mute a user
type MuteUser struct {
	UserID    int
	Reason    string
	ExpiresAt time.Time
}

// WarnUser represents the command to warn a user
type WarnUser struct {
	UserID    int
	Reason    string
	ExpiresAt time.Time
}

// DeleteWarning represents the command to delete a warning
type DeleteWarning struct {
	UserID    int
	WarningID int
}

// DeleteMute represents the command to delete a mute
type DeleteMute struct {
	UserID int
	MuteID int
}

type ExpireWarning struct {
	UserID    int
	WarningID int
}

type ExpireMute struct {
	UserID int
	MuteID int
}
