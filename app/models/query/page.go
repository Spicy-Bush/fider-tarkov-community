package query

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"

type GetPageBySlug struct {
	Slug   string
	Result *entity.Page
}

type GetPageByID struct {
	ID     int
	Result *entity.Page
}

type ListPages struct {
	View       string
	Query      string
	Topics     []string
	Tags       []string
	Status     []entity.PageStatus
	Limit      int
	Offset     int
	Result     []*entity.Page
	TotalCount int
}

type GetAllPublishedPages struct {
	Result []*entity.Page
}

type GetPageDraft struct {
	PageID int
	UserID int
	Result *entity.PageDraft
}

type GetCommentsByPage struct {
	Page   *entity.Page
	Result []*entity.Comment
}

type GetPageTopics struct {
	Result []*entity.PageTopic
}

type GetPageTags struct {
	Result []*entity.PageTag
}

type UserSubscribedToPage struct {
	PageID int
	Result bool
}

type GetPageSubscribers struct {
	PageID int
	Result []*entity.User
}

type GetPageTopicByID struct {
	ID     int
	Result *entity.PageTopic
}

type GetPageTagByID struct {
	ID     int
	Result *entity.PageTag
}
