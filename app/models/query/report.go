package query

import (
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type GetReportByID struct {
	ReportID int
	Result   *entity.Report
}

type ListReports struct {
	Status   []enum.ReportStatus
	Type     enum.ReportType
	Reason   string
	Page     int
	PerPage  int
	Result   []*entity.Report
	Total    int
}

type CountPendingReports struct {
	Result int
}

type GetReportReasons struct {
	Result []*entity.ReportReason
}

type CountUserReportsToday struct {
	UserID int
	Result int
}

type HasUserReportedTarget struct {
	UserID       int
	ReportedType enum.ReportType
	ReportedID   int
	Result       bool
}

type GetUserReportedItemsOnPost struct {
	PostID             int
	CommentIDs         []int
	HasReportedPost    bool
	ReportedCommentIDs []int
}

