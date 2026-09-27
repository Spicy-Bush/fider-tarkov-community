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

type GetUserReportStatus struct {
	PostID             int
	CommentIDs         []int
	HasReportedPost    bool  `db:"has_reported_post"`
	ReportedCommentIDs []int64 `db:"reported_comment_ids"`
	CountToday         int   `db:"count_today"`
}

type ListAllReportReasons struct {
	Result []*entity.ReportReason
}
