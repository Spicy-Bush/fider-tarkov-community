package entity

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type Report struct {
	ID             int               `json:"id"`
	ReportedType   enum.ReportType   `json:"reportedType"`
	ReportedID     int               `json:"reportedId"`
	Reason         string            `json:"reason"`
	Details        string            `json:"details,omitempty"`
	Status         enum.ReportStatus `json:"status"`
	CreatedAt      time.Time         `json:"createdAt"`
	Reporter       *User             `json:"reporter"`
	AssignedTo     *User             `json:"assignedTo,omitempty"`
	AssignedAt     *time.Time        `json:"assignedAt,omitempty"`
	ResolvedAt     *time.Time        `json:"resolvedAt,omitempty"`
	ResolvedBy     *User             `json:"resolvedBy,omitempty"`
	ResolutionNote string            `json:"resolutionNote,omitempty"`
	PostNumber     int               `json:"postNumber,omitempty"`
	PostSlug       string            `json:"postSlug,omitempty"`
}

type ReportReason struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SortOrder   int    `json:"sortOrder"`
	IsActive    bool   `json:"isActive"`
}

