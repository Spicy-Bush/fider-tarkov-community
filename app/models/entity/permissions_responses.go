package entity

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

type RolePostResponses map[enum.Role][]enum.PostStatus

type RoleResponseChange struct {
	Role    enum.Role       `json:"role"`
	Status  enum.PostStatus `json:"status"`
	Granted bool            `json:"granted"`
}

func (change *RoleResponseChange) UnmarshalJSON(content []byte) error {
	var input struct {
		Role    enum.Role        `json:"role"`
		Status  *enum.PostStatus `json:"status"`
		Granted bool             `json:"granted"`
	}
	if err := json.Unmarshal(content, &input); err != nil {
		return err
	}
	if input.Status == nil {
		return fmt.Errorf("a response status is required")
	}
	*change = RoleResponseChange{Role: input.Role, Status: *input.Status, Granted: input.Granted}
	return nil
}

var ResponseOptions = []enum.PostStatus{
	enum.PostOpen, enum.PostStarted, enum.PostCompleted,
	enum.PostDeclined, enum.PostPlanned, enum.PostDuplicate,
}

var MaxRoleResponseChanges = len(PermissionRoles) * len(ResponseOptions)

func IsResponseStatus(status enum.PostStatus) bool {
	return slices.Contains(ResponseOptions, status)
}

func (overrides RolePostResponses) ForRole(role enum.Role) []enum.PostStatus {
	if responses, exists := overrides[role]; exists {
		return responses
	}
	if collaboratorRoles.has(role) {
		return ResponseOptions
	}
	if role == enum.RoleModerator {
		return []enum.PostStatus{enum.PostDuplicate}
	}
	return []enum.PostStatus{}
}

func (overrides RolePostResponses) ByRole() map[enum.Role][]enum.PostStatus {
	result := make(map[enum.Role][]enum.PostStatus, len(PermissionRoles))
	for _, role := range PermissionRoles {
		result[role] = overrides.ForRole(role)
	}
	return result
}

func RoleResponseLock(viewer *User, authority *Tenant, role enum.Role, status enum.PostStatus) string {
	if viewer.IsAdministrator() {
		return ""
	}
	if !viewer.outranks(role) {
		return "You can only change roles below your own"
	}
	if !slices.Contains(AllowedPostResponses(viewer, authority), status) {
		return "You can only change responses you can use"
	}
	return ""
}

func RoleResponseLocks(viewer *User, authority *Tenant) map[enum.Role]map[enum.PostStatus]string {
	locks := make(map[enum.Role]map[enum.PostStatus]string, len(PermissionRoles))
	for _, role := range PermissionRoles {
		locks[role] = map[enum.PostStatus]string{}
		for _, status := range ResponseOptions {
			if reason := RoleResponseLock(viewer, authority, role, status); reason != "" {
				locks[role][status] = reason
			}
		}
	}
	return locks
}

func (overrides RolePostResponses) set(role enum.Role, status enum.PostStatus, granted bool) {
	selected := overrides.ForRole(role)
	responses := make([]enum.PostStatus, 0, len(ResponseOptions))
	for _, option := range ResponseOptions {
		allowed := slices.Contains(selected, option)
		if option == status {
			allowed = granted
		}
		if allowed {
			responses = append(responses, option)
		}
	}
	if slices.Equal(responses, RolePostResponses(nil).ForRole(role)) {
		delete(overrides, role)
	} else {
		overrides[role] = responses
	}
}

func (tenant *Tenant) ApplyRoleConfiguration(viewer *User, changes []RolePermissionChange, responses []RoleResponseChange) (RolePermissions, RolePostResponses, string) {
	next, blocked := tenant.RolePermissions.ApplyChanges(viewer, tenant, changes)
	if blocked != "" {
		return nil, nil, blocked
	}
	nextResponses := make(RolePostResponses, len(tenant.RolePostResponses))
	for role, statuses := range tenant.RolePostResponses {
		nextResponses[role] = statuses
	}

	revokedRead := make(map[enum.Role]bool)
	for _, change := range changes {
		if change.Permission == ReadResponses && !change.Granted {
			revokedRead[change.Role] = true
		}
	}
	for _, role := range PermissionRoles {
		if !revokedRead[role] {
			continue
		}
		for _, status := range nextResponses.ForRole(role) {
			if reason := RoleResponseLock(viewer, tenant, role, status); reason != "" {
				return nil, nil, reason
			}
			nextResponses.set(role, status, false)
		}
	}

	for _, granting := range []bool{false, true} {
		for _, change := range responses {
			if change.Granted != granting {
				continue
			}
			if reason := RoleResponseLock(viewer, tenant, change.Role, change.Status); reason != "" {
				return nil, nil, reason
			}
			if change.Granted && !next.Grants(change.Role, ReadResponses) {
				next, blocked = next.ApplyChanges(viewer, tenant, []RolePermissionChange{{Role: change.Role, Permission: ReadResponses, Granted: true}})
				if blocked != "" {
					return nil, nil, blocked
				}
			}
			nextResponses.set(change.Role, change.Status, change.Granted)
		}
	}
	return next, nextResponses, ""
}

func (overrides RolePostResponses) Equal(other RolePostResponses) bool {
	for _, role := range PermissionRoles {
		selected, compared := overrides.ForRole(role), other.ForRole(role)
		for _, status := range ResponseOptions {
			if slices.Contains(selected, status) != slices.Contains(compared, status) {
				return false
			}
		}
	}
	return true
}
