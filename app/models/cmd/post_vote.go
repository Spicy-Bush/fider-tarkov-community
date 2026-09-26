package cmd

type VoteOperation int

const (
	SetPostVote VoteOperation = iota
	UpvotePost
	DownvotePost
	RemovePostVote
	TogglePostVote
)

func (operation VoteOperation) Legacy() bool {
	return operation == UpvotePost || operation == DownvotePost || operation == RemovePostVote
}

type PostVoteState struct {
	Direction int   `db:"direction" json:"direction"`
	Revision  int64 `db:"revision" json:"revision"`
	Upvotes   int   `db:"upvotes" json:"upvotes"`
	Downvotes int   `db:"downvotes" json:"downvotes"`
	Applied   bool  `json:"applied"`
}

type ApplyPostVote struct {
	Number    int
	Operation VoteOperation
	Direction int
	Revision  int64

	State      PostVoteState
	Rejection  string
	Unarchived bool
}
