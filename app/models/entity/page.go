package entity

import "time"

type PageStatus string
type PageVisibility string

const (
	PageStatusDraft       PageStatus = "draft"
	PageStatusPublished   PageStatus = "published"
	PageStatusUnpublished PageStatus = "unpublished"
	PageStatusScheduled   PageStatus = "scheduled"

	PageVisibilityPublic   PageVisibility = "public"
	PageVisibilityPrivate  PageVisibility = "private"
	PageVisibilityUnlisted PageVisibility = "unlisted"
)

type Page struct {
	ID              int              `json:"id"`
	Title           string           `json:"title"`
	Slug            string           `json:"slug"`
	Content         string           `json:"content"`
	Excerpt         string           `json:"excerpt,omitempty"`
	BannerImageBKey string           `json:"bannerImageBKey,omitempty"`
	Status          PageStatus       `json:"status"`
	Visibility      PageVisibility   `json:"visibility"`
	AllowedRoles    []string         `json:"allowedRoles,omitempty"`
	ParentPageID    *int             `json:"parentPageId,omitempty"`
	AllowComments   bool             `json:"allowComments"`
	AllowReactions  bool             `json:"allowReactions"`
	ShowTOC         bool             `json:"showToc"`
	ScheduledFor    *time.Time       `json:"scheduledFor,omitempty"`
	PublishedAt     *time.Time       `json:"publishedAt,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	CreatedBy       *User            `json:"createdBy"`
	UpdatedBy       *User            `json:"updatedBy"`
	Authors         []*User          `json:"authors,omitempty"`
	Topics          []*PageTopic     `json:"topics,omitempty"`
	Tags            []*PageTag       `json:"tags,omitempty"`
	MetaDescription string           `json:"metaDescription,omitempty"`
	CanonicalURL    string           `json:"canonicalUrl,omitempty"`
	CommentsCount   int              `json:"commentsCount"`
	ReactionCounts  []ReactionCounts `json:"reactionCounts,omitempty"`
	EmbeddedPosts   []*Post          `json:"embeddedPosts"`
	CachedAt        *time.Time       `json:"cachedAt,omitempty"`
}

type PageTopic struct {
	ID          int    `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Slug        string `json:"slug" db:"slug"`
	Description string `json:"description,omitempty" db:"description"`
	Color       string `json:"color,omitempty" db:"color"`
}

type PageTag struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
	Slug string `json:"slug" db:"slug"`
}

type PageDraft struct {
	ID              int       `json:"id"`
	PageID          int       `json:"pageId"`
	UserID          int       `json:"userId"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	Content         string    `json:"content"`
	Excerpt         string    `json:"excerpt,omitempty"`
	BannerImageBKey string    `json:"bannerImageBKey,omitempty"`
	MetaDescription string    `json:"metaDescription,omitempty"`
	ShowTOC         bool      `json:"showToc"`
	DraftData       string    `json:"draftData"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
