package enum

import "fmt"

type VisualRole int

const (
	VisualRoleNone          VisualRole = 0
	VisualRoleVisitor       VisualRole = 1
	VisualRoleHelper        VisualRole = 2
	VisualRoleAdministrator VisualRole = 3
	VisualRoleModerator     VisualRole = 4
	VisualRoleBSGCrew       VisualRole = 5
	VisualRoleDeveloper     VisualRole = 6
	VisualRoleSherpa        VisualRole = 7
	VisualRoleTCStaff       VisualRole = 8
	VisualRoleEmissary      VisualRole = 9
)

var visualRoleNames = []string{
	"",
	"Visitor",
	"Helper",
	"Administrator",
	"Moderator",
	"BSGCrew",
	"Developer",
	"Sherpa",
	"TCStaff",
	"Emissary",
}

func (r VisualRole) String() string {
	if r >= 0 && int(r) < len(visualRoleNames) {
		return visualRoleNames[r]
	}
	return ""
}

func (r VisualRole) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, r.String())), nil
}

func (r *VisualRole) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	for i, name := range visualRoleNames {
		if name == s {
			*r = VisualRole(i)
			return nil
		}
	}
	*r = VisualRoleNone
	return nil
}

func (r VisualRole) IsValid() bool {
	return r >= 0 && int(r) < len(visualRoleNames)
}

var AllVisualRoles = []VisualRole{
	VisualRoleVisitor,
	VisualRoleHelper,
	VisualRoleAdministrator,
	VisualRoleModerator,
	VisualRoleBSGCrew,
	VisualRoleDeveloper,
	VisualRoleSherpa,
	VisualRoleTCStaff,
	VisualRoleEmissary,
}
