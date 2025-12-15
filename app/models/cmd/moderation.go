package cmd

type SetModerationPending struct {
	ContentType    string
	ContentID      int
	Pending        bool
	ModerationData string
}

