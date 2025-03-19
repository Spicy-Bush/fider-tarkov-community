package enum

import "strconv"

type VisualRole int

const (
	// Just use the role we have in the app/models/enum/user_role.go
	VisualRoleNone          VisualRole = 0
	VisualRoleVisitor       VisualRole = 1
	VisualRoleHelpful       VisualRole = 2
	VisualRoleAdministrator VisualRole = 3
	VisualRoleModerator     VisualRole = 4
	VisualRoleBSGCrew       VisualRole = 5
	VisualRoleDeveloper     VisualRole = 6
	VisualRoleSherpa        VisualRole = 7
	VisualRoleTCStaff       VisualRole = 8
	VisualRoleEmissary      VisualRole = 9
)

// String returns string representation of role
func (r VisualRole) String() string {
	switch r {
	case VisualRoleVisitor:
		return "Visitor"
	case VisualRoleHelpful:
		return "Helpful"
	case VisualRoleAdministrator:
		return "Administrator"
	case VisualRoleModerator:
		return "Moderator"
	case VisualRoleBSGCrew:
		return "BSGCrew"
	case VisualRoleDeveloper:
		return "Developer"
	case VisualRoleSherpa:
		return "Sherpa"
	case VisualRoleTCStaff:
		return "TCStaff"
	case VisualRoleEmissary:
		return "Emissary"
	default:
		return ""
	}
}

func (r VisualRole) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

func (r *VisualRole) UnmarshalText(text []byte) error {
	val, err := strconv.Atoi(string(text))
	if err != nil {
		return err
	}
	*r = VisualRole(val)
	return nil
}

func (r VisualRole) IsValid() bool {
	return r == VisualRoleNone ||
		r == VisualRoleVisitor ||
		r == VisualRoleHelpful ||
		r == VisualRoleAdministrator ||
		r == VisualRoleModerator ||
		r == VisualRoleBSGCrew ||
		r == VisualRoleDeveloper ||
		r == VisualRoleSherpa ||
		r == VisualRoleTCStaff ||
		r == VisualRoleEmissary
}

var AllVisualRoles = []VisualRole{
	VisualRoleVisitor,
	VisualRoleHelpful,
	VisualRoleAdministrator,
	VisualRoleModerator,
	VisualRoleBSGCrew,
	VisualRoleDeveloper,
	VisualRoleSherpa,
	VisualRoleTCStaff,
	VisualRoleEmissary,
}
