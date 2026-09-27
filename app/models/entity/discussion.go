package entity

import (
	"fmt"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type DiscussionOwner struct {
	Kind   string `json:"kind"`
	ID     int    `json:"id"`
	Number int    `json:"number,omitempty"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

type Discussion struct {
	Owner          DiscussionOwner `json:"owner"`
	PostStatus     enum.PostStatus `json:"-"`
	Locked         bool            `json:"-"`
	PageStatus     PageStatus      `json:"-"`
	Visibility     PageVisibility  `json:"-"`
	AllowedRoles   []string        `json:"-"`
	AllowComments  bool            `json:"-"`
	AllowImages    bool            `json:"-"`
	AllowReactions bool            `json:"-"`

	postAuthorID int
	postHidden   bool
}

func PostDiscussion(post *Post) *Discussion {
	discussion := &Discussion{
		PostStatus:     post.Status,
		postHidden:     post.ModerationPending,
		Locked:         post.IsLocked(),
		AllowComments:  true,
		AllowImages:    true,
		AllowReactions: true,
		Owner: DiscussionOwner{
			Kind:   "post",
			ID:     post.ID,
			Number: post.Number,
			Title:  post.Title,
			URL:    fmt.Sprintf("/posts/%d/%s", post.Number, post.Slug),
		},
	}
	if post.User != nil {
		discussion.postAuthorID = post.User.ID
	}

	return discussion
}

func PageDiscussion(page *Page) *Discussion {
	return &Discussion{
		PageStatus:     page.Status,
		Visibility:     page.Visibility,
		AllowedRoles:   page.AllowedRoles,
		AllowComments:  page.AllowComments,
		AllowImages:    page.AllowCommentImages,
		AllowReactions: page.AllowReactions,
		Owner: DiscussionOwner{
			Kind:  "page",
			ID:    page.ID,
			Title: page.Title,
			URL:   "/pages/" + page.Slug,
		},
	}
}

func (discussion *Discussion) CanView(user *User) bool {
	if discussion.Owner.Kind == "page" {
		return canViewPage(discussion.PageStatus, discussion.Visibility, discussion.AllowedRoles, user)
	}

	return canViewPost(discussion.PostStatus, discussion.postHidden, discussion.postAuthorID, user)
}

func canViewPost(status enum.PostStatus, hidden bool, authorID int, user *User) bool {
	if status == enum.PostDeleted {
		return false
	}
	if !hidden {
		return true
	}

	return user != nil && (user.ID == authorID || user.IsCollaborator() || user.IsModerator())
}

type DiscussionPermissions struct {
	Comment bool `json:"comment"`
	React   bool `json:"react"`
	Images  bool `json:"images"`
}

func (discussion *Discussion) Permissions(user *User, tenant *Tenant) DiscussionPermissions {
	permissions := DiscussionPermissions{}
	if !discussion.CanView(user) {
		return permissions
	}

	permissions.Images = discussion.AllowImages
	if !canAct(user, tenant) || user.IsMuted() {
		return permissions
	}

	permissions.Comment = discussion.AllowComments
	permissions.React = discussion.AllowReactions

	if discussion.Locked && !user.IsCollaborator() {
		permissions.Comment = false
		permissions.React = false
	}

	if tenant == nil {
		return permissions
	}

	settings := tenant.GeneralSettings
	if settings == nil {
		return permissions
	}

	if settings.CommentingGloballyDisabled && !user.IsCollaborator() {
		permissions.Comment = false
	}

	for _, role := range settings.CommentingDisabledFor {
		if role == user.Role.String() {
			permissions.Comment = false
		}
	}

	return permissions
}

type CommentPermissions struct {
	Edit     bool `json:"edit"`
	Delete   bool `json:"delete"`
	Moderate bool `json:"moderate"`
	Reply    bool `json:"reply"`
	React    bool `json:"react"`
	Report   bool `json:"report"`
}

func (comment *Comment) AllowedActions(user *User, discussion *Discussion, tenant *Tenant, now time.Time) CommentPermissions {
	permissions := CommentPermissions{}
	if !canAct(user, tenant) || !discussion.CanView(user) {
		return permissions
	}

	author := comment.User
	own := author != nil && author.ID == user.ID
	canModerate := comment.canModerate(user)
	permissions.Delete = own || canModerate
	if comment.Deleted {
		return permissions
	}

	canEdit := canModerate || (own && !now.After(comment.CreatedAt.Add(time.Hour)))
	var settings *GeneralSettings
	if tenant != nil {
		settings = tenant.GeneralSettings
	}

	if settings != nil && settings.CommentingGloballyDisabled && !user.IsCollaborator() {
		canEdit = false
	}

	if discussion.Locked && !user.IsCollaborator() {
		canEdit = false
	}

	permissions.Edit = canEdit && !user.IsMuted()
	permissions.Moderate = canModerate

	discussionPermissions := discussion.Permissions(user, tenant)
	permissions.Reply = discussionPermissions.Comment
	permissions.React = discussionPermissions.React && (!comment.ModerationPending || own)
	permissions.Report = !comment.ModerationPending && !own && (settings == nil || !settings.ReportingGloballyDisabled)
	return permissions
}

func (comment *Comment) canModerate(user *User) bool {
	if !canAct(user, nil) {
		return false
	}

	author := comment.User
	moderatorTarget := author != nil && (author.ID == user.ID || author.Role == enum.RoleVisitor || author.Role == enum.RoleHelper)
	return user.IsCollaborator() || (user.IsModerator() && moderatorTarget)
}

func (comment *Comment) ContentState(user *User, discussion *Discussion) string {
	if comment.Deleted {
		return "deleted"
	}

	own := user != nil && comment.User != nil && user.ID == comment.User.ID
	if !discussion.CanView(user) || (comment.ModerationPending && !own && !comment.canModerate(user)) {
		return "hidden"
	}

	return "visible"
}

func (comment *Comment) ForViewer(user *User, discussion *Discussion, tenant *Tenant, now time.Time) *Comment {
	visible := *comment
	visible.Permissions = comment.AllowedActions(user, discussion, tenant, now)
	visible.State = comment.ContentState(user, discussion)
	visible.ModerationPending = false
	visible.ModerationData = ""

	if visible.Permissions.Moderate {
		visible.ModerationPending = comment.ModerationPending
		visible.ModerationData = comment.ModerationData
	}

	if visible.State != "visible" {
		visible.Content = ""
		visible.User = nil
		visible.Attachments = nil
		visible.EditedAt = nil
		visible.EditedBy = nil
		visible.ReactionCounts = nil
		visible.ModerationData = ""
		visible.ModerationPending = false
		visible.Permissions = CommentPermissions{}
	}

	return &visible
}

type DiscussionPage struct {
	Owner       DiscussionOwner       `json:"owner"`
	Permissions DiscussionPermissions `json:"permissions"`
	Comments    []*Comment            `json:"comments"`
	Replies     []*Comment            `json:"replies,omitempty"`
	Next        string                `json:"next,omitempty"`
}

type CommentContext struct {
	DiscussionPage
	CommentID      int  `json:"commentId"`
	NextAncestorID *int `json:"nextAncestorId,omitempty"`
}
