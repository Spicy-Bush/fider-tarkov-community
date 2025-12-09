package cmd

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type CreateReport struct {
	ReportedType enum.ReportType
	ReportedID   int
	Reason       string
	Details      string
	Result       int
}

type AssignReport struct {
	ReportID   int
	AssignToID int
}

type UnassignReport struct {
	ReportID int
}

type ResolveReport struct {
	ReportID       int
	Status         enum.ReportStatus
	ResolutionNote string
}

type DeleteReport struct {
	ReportID int
}

type CreateReportReason struct {
	Title       string
	Description string
	Result      int
}

type UpdateReportReason struct {
	ID          int
	Title       string
	Description string
	IsActive    bool
}

type DeleteReportReason struct {
	ID int
}

type ReorderReportReasons struct {
	IDs []int
}
