package entity_test

import (
	"encoding/json"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
)

func TestUserJSONVisualRolesAndPrivateFields(t *testing.T) {
	defaults := []struct {
		role   enum.Role
		visual enum.VisualRole
	}{
		{enum.RoleVisitor, enum.VisualRoleVisitor},
		{enum.RoleHelper, enum.VisualRoleHelper},
		{enum.RoleModerator, enum.VisualRoleModerator},
		{enum.RoleCollaborator, enum.VisualRoleBSGCrew},
		{enum.RoleAdministrator, enum.VisualRoleAdministrator},
	}

	for _, defaults := range defaults {
		t.Run(defaults.role.String(), func(t *testing.T) {
			user := entity.User{
				ID:        1,
				Name:      "John Doe",
				Email:     "johndoe@example.com",
				Role:      defaults.role,
				Status:    enum.UserActive,
				Providers: []*entity.UserProvider{{Name: "fixture", UID: "private-provider-id"}},
			}

			for _, override := range []enum.VisualRole{enum.VisualRoleNone, enum.VisualRoleSherpa} {
				user.VisualRole = override
				want := defaults.visual
				if override != enum.VisualRoleNone {
					want = override
				}

				for _, includePrivate := range []bool{false, true} {
					var value any = user
					if includePrivate {
						value = entity.UserWithEmail{User: &user}
					}

					data, err := json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}

					var result map[string]any
					if err := json.Unmarshal(data, &result); err != nil {
						t.Fatal(err)
					}

					if result["visualRole"] != want.String() || result["role"] != defaults.role.String() {
						t.Fatalf("incorrect effective badge or permission role: %s", data)
					}

					if result["id"] != float64(user.ID) || result["name"] != user.Name || result["permissions"] == nil {
						t.Fatalf("user fields were lost during serialization: %s", data)
					}

					for _, field := range []string{"email", "providers", "visualRoleOverride"} {
						if _, present := result[field]; present != includePrivate {
							t.Errorf("private=%t: field %s present=%t", includePrivate, field, present)
						}
					}

					if includePrivate && (result["email"] != user.Email || result["visualRoleOverride"] != override.String()) {
						t.Fatalf("editor lost email or configured override: %s", data)
					}
				}

				if user.VisualRole != override {
					t.Fatal("serializing the effective badge changed the stored override")
				}
			}
		})
	}
}
