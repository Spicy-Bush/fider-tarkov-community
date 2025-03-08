package enum

type VoteType int

const (
	// VoteTypeUp represents an upvote
	VoteTypeUp VoteType = 1
	// VoteTypeDown represents a downvote
	VoteTypeDown VoteType = -1
)
