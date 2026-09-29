package entity

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type PostPermissions struct {
	Edit      bool              `json:"edit"`
	Delete    bool              `json:"delete"`
	Respond   []enum.PostStatus `json:"respond"`
	Lock      bool              `json:"lock"`
	Archive   bool              `json:"archive"`
	Moderate  bool              `json:"moderate"`
	Tag       bool              `json:"tag"`
	Report    bool              `json:"report"`
	ViewVotes bool              `json:"viewVotes"`
	Vote      bool              `json:"vote"`
	Follow    bool              `json:"follow"`
}

func (post *Post) AllowedActions(user *User, tenant *Tenant, now time.Time) PostPermissions {
	permissions := PostPermissions{Respond: []enum.PostStatus{}}
	if post == nil || !CanAct(user, tenant) {
		return permissions
	}

	authorID := 0
	if post.User != nil {
		authorID = post.User.ID
	}
	if !canViewPost(post.Status, post.ModerationPending, authorID, user, tenant) {
		return permissions
	}

	permissions.Respond = AllowedPostResponses(user, tenant)
	permissions.Lock = Can(user, tenant, LockPosts)
	permissions.Archive = Can(user, tenant, ManageArchive)
	permissions.Moderate = Can(user, tenant, ModeratePosts)
	permissions.ViewVotes = Can(user, tenant, ViewPostVotes)
	permissions.Tag = Can(user, tenant, TagPosts)
	permissions.Follow = !post.IsLocked() || permissions.Lock
	permissions.Vote = permissions.Follow && post.CanBeVoted() && !user.IsMuted()

	if post.User != nil {
		own := post.User.ID == user.ID
		canManage := canManageContentRole(user, post.User.Role)

		editOthers := canManage && (staffRoles.has(user.Role) || Can(user, tenant, ModeratePosts))
		permissions.Edit = editOthers || (own && !now.After(post.CreatedAt.Add(time.Hour)))
		permissions.Delete = Can(user, tenant, DeletePosts) && canManage
		permissions.Report = !own
	} else {
		permissions.Edit = canManageContentRole(user, 0)
		permissions.Delete = Can(user, tenant, DeletePosts) && permissions.Edit
	}

	if post.IsLocked() && !permissions.Lock {
		permissions.Edit = false
	}
	permissions.Edit = permissions.Edit && Can(user, tenant, EditPosts)

	if tenant != nil && tenant.GeneralSettings != nil {
		settings := tenant.GeneralSettings
		if settings.PostingGloballyDisabled && !CanBypassContentRestrictions(user, tenant) {
			permissions.Edit = false
		}

		if settings.ReportingGloballyDisabled {
			permissions.Report = false
		}
	}

	if !Can(user, tenant, TagPostsOutsideWindow) {
		permissions.Tag = permissions.Tag && post.withinTaggingWindow(now)
	}

	return permissions
}

func AllowedPostResponses(user *User, tenant *Tenant) []enum.PostStatus {
	if !Can(user, tenant, ReadResponses) {
		return []enum.PostStatus{}
	}
	var responses RolePostResponses
	if tenant != nil {
		responses = tenant.RolePostResponses
	}
	return responses.ForRole(user.Role)
}

func (post *Post) withinTaggingWindow(now time.Time) bool {
	if now.After(post.CreatedAt.Add(7 * 24 * time.Hour)) {
		return false
	}

	return post.FirstTaggedAt == nil || !now.After(post.FirstTaggedAt.Add(24*time.Hour))
}

type TagPermissions struct {
	Assign bool `json:"assign"`
}

func DefaultQueueDate(user *User, tenant *Tenant) string {
	if Can(user, tenant, ManageQueue) && !Can(user, tenant, TagPostsOutsideWindow) {
		return "7d"
	}

	return ""
}

func (tag *Tag) AllowedActions(user *User, tenant *Tenant) TagPermissions {
	return TagPermissions{
		Assign: Can(user, tenant, TagPosts) && (tag.IsPublic || Can(user, tenant, ViewPrivateTags)),
	}
}
