package enum

type ReportType int

const (
	ReportTypePost    ReportType = 1
	ReportTypeComment ReportType = 2
)

var reportTypeIDs = map[ReportType]string{
	ReportTypePost:    "post",
	ReportTypeComment: "comment",
}

var reportTypeNames = map[string]ReportType{
	"post":    ReportTypePost,
	"comment": ReportTypeComment,
}

func (t ReportType) String() string {
	return reportTypeIDs[t]
}

func (t ReportType) MarshalText() ([]byte, error) {
	return []byte(reportTypeIDs[t]), nil
}

func (t *ReportType) UnmarshalText(text []byte) error {
	*t = reportTypeNames[string(text)]
	return nil
}

type ReportStatus int

const (
	ReportStatusPending   ReportStatus = 1
	ReportStatusInReview  ReportStatus = 2
	ReportStatusResolved  ReportStatus = 3
	ReportStatusDismissed ReportStatus = 4
)

var reportStatusIDs = map[ReportStatus]string{
	ReportStatusPending:   "pending",
	ReportStatusInReview:  "in_review",
	ReportStatusResolved:  "resolved",
	ReportStatusDismissed: "dismissed",
}

var reportStatusNames = map[string]ReportStatus{
	"pending":   ReportStatusPending,
	"in_review": ReportStatusInReview,
	"resolved":  ReportStatusResolved,
	"dismissed": ReportStatusDismissed,
}

func (s ReportStatus) String() string {
	return reportStatusIDs[s]
}

func (s ReportStatus) MarshalText() ([]byte, error) {
	return []byte(reportStatusIDs[s]), nil
}

func (s *ReportStatus) UnmarshalText(text []byte) error {
	*s = reportStatusNames[string(text)]
	return nil
}

