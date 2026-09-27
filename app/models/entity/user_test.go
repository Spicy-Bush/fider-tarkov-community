package entity_test

import (
	"encoding/json"
	"testing"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
)

const emptyUserPermissions = `"permissions":{"readProfile":false,"editName":false,"editAvatar":false,` +
	`"block":false,"moderate":false,"deleteModeration":false,"expireModeration":false,` +
	`"changeRole":false,"changeVisualRole":false}`

func TestUserWithEmail_MarshalJSON(t *testing.T) {

	RegisterT(t)
	user := entity.UserWithEmail{
		User: &entity.User{
			ID:     1,
			Name:   "John Doe",
			Email:  "johndoe@example.com",
			Role:   1,
			Status: 1,
		},
	}

	expectedJSON := `{"id":1,"name":"John Doe","role":"visitor","status":"active",` + emptyUserPermissions +
		`,"email":"johndoe@example.com","visualRole":"","providers":[]}`

	jsonData, err := json.Marshal(user)
	if err != nil {
		t.Errorf("Failed to marshal user to JSON: %v", err)
	}

	Expect(string(jsonData)).Equals(expectedJSON)

}

func TestUser_MarshalJSON(t *testing.T) {

	RegisterT(t)
	user := entity.User{
		ID:     1,
		Name:   "John Doe",
		Email:  "johndoe@example.com",
		Role:   1,
		Status: 1,
	}

	expectedJSON := `{"id":1,"name":"John Doe","role":"visitor","visualRole":"","status":"active",` + emptyUserPermissions + `}`

	jsonData, err := json.Marshal(user)
	if err != nil {
		t.Errorf("Failed to marshal user to JSON: %v", err)
	}

	Expect(string(jsonData)).Equals(expectedJSON)

}
