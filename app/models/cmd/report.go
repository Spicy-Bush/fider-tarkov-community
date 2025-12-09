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

