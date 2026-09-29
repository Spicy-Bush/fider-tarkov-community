package query

type GetPostAttachments struct {
	PostID int
	Result []string
}

type CanReadAttachment struct {
	Key     string
	Result  bool
	Version string
}
