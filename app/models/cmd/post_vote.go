package cmd

import "time"

type PostVoteState struct {
	Direction      int       `db:"direction" json:"direction"`
	Revision       int64     `db:"revision" json:"revision"`
	Upvotes        int       `db:"upvotes" json:"upvotes"`
	Downvotes      int       `db:"downvotes" json:"downvotes"`
	LastActivityAt time.Time `db:"last_activity_at" json:"lastActivityAt"`
	Applied        bool      `json:"applied"`
}

type ApplyPostVote struct {
	Number    int
	Direction int
	Revision  int64

	State      PostVoteState
	Unarchived bool
}
