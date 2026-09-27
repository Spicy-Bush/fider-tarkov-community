package entity

type NotificationDelivery struct {
	Post    *Post                `json:"post,omitempty"`
	Comment *CommentNotification `json:"comment,omitempty"`
}

type CommentNotification struct {
	CommentID      int             `json:"commentId"`
	Owner          DiscussionOwner `json:"owner"`
	Content        string          `json:"content"`
	MentionIDs     []int           `json:"mentionIds"`
	ParentAuthorID int             `json:"parentAuthorId"`
	Edited         bool            `json:"edited"`
}
