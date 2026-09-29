package query

type RealtimeViewer struct {
	TenantID int
	UserID   int
	PageID   int
}

type RealtimeAccess struct {
	Reports bool
	Queue   bool
	Pages   bool
}

type GetRealtimeAccess struct {
	Viewers []RealtimeViewer
	Result  []RealtimeAccess
}
