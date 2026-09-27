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
	if post == nil || !canAct(user, tenant) {
		return permissions
	}

	authorID := 0
	if post.User != nil {
		authorID = post.User.ID
	}
	if !canViewPost(post.Status, post.ModerationPending, authorID, user) {
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
		moderatorTarget := post.User.Role == enum.RoleVisitor || post.User.Role == enum.RoleHelper
		canManage := user.IsCollaborator() || (user.IsModerator() && moderatorTarget)

		permissions.Edit = canManage || (own && !now.After(post.CreatedAt.Add(time.Hour)))
		permissions.Delete = Can(user, tenant, DeletePosts) && canManage
		permissions.Report = !own
	} else {
		permissions.Edit = user.IsCollaborator()
		permissions.Delete = Can(user, tenant, DeletePosts) && user.IsCollaborator()
	}

	if post.IsLocked() && !permissions.Lock {
		permissions.Edit = false
	}
	permissions.Edit = permissions.Edit && Can(user, tenant, EditPosts)

	if tenant != nil && tenant.GeneralSettings != nil {
		settings := tenant.GeneralSettings
		if settings.PostingGloballyDisabled && !user.IsCollaborator() {
			permissions.Edit = false
		}

		if settings.ReportingGloballyDisabled {
			permissions.Report = false
		}
	}

	if user.IsHelper() {
		permissions.Tag = permissions.Tag && post.withinTaggingWindow(now)
	}

	return permissions
}

func AllowedPostResponses(user *User, tenant *Tenant) []enum.PostStatus {
	if !Can(user, tenant, RespondToPosts) {
		return []enum.PostStatus{}
	}

	if user.IsModerator() {
		return []enum.PostStatus{enum.PostDuplicate}
	}

	return []enum.PostStatus{
		enum.PostOpen,
		enum.PostStarted,
		enum.PostCompleted,
		enum.PostDeclined,
		enum.PostPlanned,
		enum.PostDuplicate,
	}
}

func (post *Post) withinTaggingWindow(now time.Time) bool {
	if now.After(post.CreatedAt.Add(7 * 24 * time.Hour)) {
		return false
	}

	return post.FirstTaggedAt == nil || !now.After(post.FirstTaggedAt.Add(24 * time.Hour))
}

type TagPermissions struct {
	Assign bool `json:"assign"`
}

func DefaultQueueDate(user *User) string {
	if user != nil && user.IsHelper() {
		return "7d"
	}

	return ""
}

func (tag *Tag) AllowedActions(user *User, tenant *Tenant) TagPermissions {
	return TagPermissions{
		Assign: Can(user, tenant, TagPosts) && (tag.IsPublic || !user.IsHelper()),
	}
}
