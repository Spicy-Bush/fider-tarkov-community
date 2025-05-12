package query

import (
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

type CountUsers struct {
	Result int
}

type UserSubscribedTo struct {
	PostID int

	Result bool
}

type GetUserByAPIKey struct {
	APIKey string

	Result *entity.User
}

type GetCurrentUserSettings struct {
	Result map[string]string
}

type GetUserByID struct {
	UserID int

	Result *entity.User
}

type GetUserByEmail struct {
	Email string

	Result *entity.User
}

type GetUserByProvider struct {
	Provider string
	UID      string

	Result *entity.User
}

type GetAllUsers struct {
	Result []*entity.User
}

type GetAllUserProviders struct {
	Result []*entity.UserProvider
}

type GetAllUsersNames struct {
	Query  string
	Limit  int
	Result []*dto.UserNames
}

// GetUserProfileStats returns the user's activity stats
type GetUserProfileStats struct {
	UserID int
	Result struct {
		Posts    int `json:"posts"`
		Comments int `json:"comments"`
		Votes    int `json:"votes"`
	}
}

// GetUserProfileStanding returns the user's standing (warnings, mutes)
type GetUserProfileStanding struct {
	UserID int
	Result struct {
		Warnings []struct {
			ID        int        `json:"id"`
			Reason    string     `json:"reason"`
			CreatedAt time.Time  `json:"createdAt"`
			ExpiresAt *time.Time `json:"expiresAt,omitempty"`
		} `json:"warnings"`
		Mutes []struct {
			ID        int        `json:"id"`
			Reason    string     `json:"reason"`
			CreatedAt time.Time  `json:"createdAt"`
			ExpiresAt *time.Time `json:"expiresAt,omitempty"`
		} `json:"mutes"`
	}
}

// UserPostResult represents a post in the search result
type UserPostResult struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
}

// UserCommentResult represents a comment in the search result
type UserCommentResult struct {
	ID        int       `json:"id"`
	Content   string    `json:"content"`
	PostID    int       `json:"postId"`
	PostTitle string    `json:"postTitle"`
	CreatedAt time.Time `json:"createdAt"`
}

// SearchUserContent searches through the user's content
type SearchUserContent struct {
	UserID int
	Query  string
	Result struct {
		Posts    []UserPostResult    `json:"posts"`
		Comments []UserCommentResult `json:"comments"`
	}
}

type UpdateUser struct {
	User *entity.User
}
