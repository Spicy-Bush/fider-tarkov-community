package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

type dbUser struct {
	ID            sql.NullInt64  `db:"id"`
	Name          sql.NullString `db:"name"`
	Email         sql.NullString `db:"email"`
	Tenant        *dbTenant      `db:"tenant"`
	Role          sql.NullInt64  `db:"role"`
	VisualRole    sql.NullInt64  `db:"visual_role"`
	Status        sql.NullInt64  `db:"status"`
	AvatarType    sql.NullInt64  `db:"avatar_type"`
	AvatarBlobKey sql.NullString `db:"avatar_bkey"`
	Providers     []*dbUserProvider
}

type dbUserProvider struct {
	Name sql.NullString `db:"provider"`
	UID  sql.NullString `db:"provider_uid"`
}

// dbUserWarning represents a user warning in the database
type dbUserWarning struct {
	ID        int          `db:"id"`
	Reason    string       `db:"reason"`
	CreatedAt time.Time    `db:"created_at"`
	ExpiresAt sql.NullTime `db:"expires_at"`
}

// dbUserMute represents a user mute in the database
type dbUserMute struct {
	ID        int          `db:"id"`
	Reason    string       `db:"reason"`
	CreatedAt time.Time    `db:"created_at"`
	ExpiresAt sql.NullTime `db:"expires_at"`
}

// dbUserPost represents a post in the database for user content search
type dbUserPost struct {
	ID        int       `db:"id"`
	Title     string    `db:"title"`
	CreatedAt time.Time `db:"created_at"`
}

// dbUserComment represents a comment in the database for user content search
type dbUserComment struct {
	ID         int       `db:"id"`
	Content    string    `db:"content"`
	PostID     int       `db:"post_id"`
	PostNumber int       `db:"post_number"`
	PostTitle  string    `db:"post_title"`
	CreatedAt  time.Time `db:"created_at"`
}

func (u *dbUser) toModel(ctx context.Context) *entity.User {
	if u == nil {
		return nil
	}

	avatarURL := ""
	avatarType := enum.AvatarType(u.AvatarType.Int64)
	if u.AvatarType.Valid {
		avatarURL = buildAvatarURL(ctx, avatarType, int(u.ID.Int64), u.Name.String, u.AvatarBlobKey.String)
	}

	user := &entity.User{
		ID:            int(u.ID.Int64),
		Name:          u.Name.String,
		Email:         u.Email.String,
		Tenant:        u.Tenant.toModel(),
		Role:          enum.Role(u.Role.Int64),
		VisualRole:    enum.VisualRole(u.VisualRole.Int64),
		Providers:     make([]*entity.UserProvider, len(u.Providers)),
		Status:        enum.UserStatus(u.Status.Int64),
		AvatarType:    avatarType,
		AvatarBlobKey: u.AvatarBlobKey.String,
		AvatarURL:     avatarURL,
	}

	for i, p := range u.Providers {
		user.Providers[i] = &entity.UserProvider{
			Name: p.Name.String,
			UID:  p.UID.String,
		}
	}

	return user
}

type dbUserSetting struct {
	Key   string `db:"key"`
	Value string `db:"value"`
}

func countUsers(ctx context.Context, q *query.CountUsers) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var count int
		err := trx.Scalar(&count, "SELECT COUNT(*) FROM users WHERE tenant_id = $1", tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to count users")
		}
		q.Result = count
		return nil
	})
}

func blockUser(ctx context.Context, c *cmd.BlockUser) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := trx.Execute(
			"UPDATE users SET status = $3 WHERE id = $1 AND tenant_id = $2",
			c.UserID, tenant.ID, enum.UserBlocked,
		); err != nil {
			return errors.Wrap(err, "failed to block user")
		}
		return nil
	})
}

func unblockUser(ctx context.Context, c *cmd.UnblockUser) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := trx.Execute(
			"UPDATE users SET status = $3 WHERE id = $1 AND tenant_id = $2",
			c.UserID, tenant.ID, enum.UserActive,
		); err != nil {
			return errors.Wrap(err, "failed to unblock user")
		}
		return nil
	})
}

func deleteCurrentUser(ctx context.Context, c *cmd.DeleteCurrentUser) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if _, err := trx.Execute(
			"UPDATE users SET role = $3, status = $4, name = '', email = '', api_key = null, api_key_date = null WHERE id = $1 AND tenant_id = $2",
			user.ID, tenant.ID, enum.RoleVisitor, enum.UserDeleted,
		); err != nil {
			return errors.Wrap(err, "failed to delete current user")
		}

		var tables = []struct {
			name       string
			userColumn string
		}{
			{"user_providers", "user_id"},
			{"user_settings", "user_id"},
			{"notifications", "user_id"},
			{"notifications", "author_id"},
			{"post_votes", "user_id"},
			{"post_subscribers", "user_id"},
			{"email_verifications", "user_id"},
		}

		for _, table := range tables {
			if _, err := trx.Execute(
				fmt.Sprintf("DELETE FROM %s WHERE %s = $1 AND tenant_id = $2", table.name, table.userColumn),
				user.ID, tenant.ID,
			); err != nil {
				return errors.Wrap(err, "failed to delete current user's %s records", table)
			}
		}

		return nil
	})
}

func regenerateAPIKey(ctx context.Context, c *cmd.RegenerateAPIKey) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		apiKey := entity.GenerateEmailVerificationKey()

		if _, err := trx.Execute(
			"UPDATE users SET api_key = $3, api_key_date = $4 WHERE id = $1 AND tenant_id = $2",
			user.ID, tenant.ID, apiKey, time.Now(),
		); err != nil {
			return errors.Wrap(err, "failed to update current user's API Key")
		}

		c.Result = apiKey
		return nil
	})
}

func getUserByAPIKey(ctx context.Context, q *query.GetUserByAPIKey) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		result, err := queryUser(ctx, trx, "api_key = $1 AND tenant_id = $2", q.APIKey, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get user with API Key '%s'", q.APIKey)
		}
		q.Result = result
		return nil
	})
}

func userSubscribedTo(ctx context.Context, q *query.UserSubscribedTo) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user == nil {
			q.Result = false
			return nil
		}

		var status int
		err := trx.Scalar(&status, "SELECT status FROM post_subscribers WHERE user_id = $1 AND post_id = $2", user.ID, q.PostID)
		if err != nil && errors.Cause(err) != app.ErrNotFound {
			return errors.Wrap(err, "failed to get subscription status")
		}

		if errors.Cause(err) == app.ErrNotFound {
			for _, e := range enum.AllNotificationEvents {
				for _, r := range e.RequiresSubscriptionUserRoles {
					if r == user.Role {
						q.Result = false
						return nil
					}
				}
			}
			q.Result = true
			return nil
		}

		if status == 1 {
			q.Result = true
			return nil
		}

		q.Result = false
		return nil
	})
}

func changeUserRole(ctx context.Context, c *cmd.ChangeUserRole) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		cmd := "UPDATE users SET role = $3 WHERE id = $1 AND tenant_id = $2"
		_, err := trx.Execute(cmd, c.UserID, tenant.ID, c.Role)
		if err != nil {
			return errors.Wrap(err, "failed to change user's role")
		}
		return nil
	})
}

func changeUserVisualRole(ctx context.Context, c *cmd.ChangeUserVisualRole) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute(`
			UPDATE users 
			SET visual_role = $1
			WHERE id = $2 AND tenant_id = $3
		`, c.VisualRole, c.UserID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to update user's visual role")
		}
		return nil
	})
}

func changeUserEmail(ctx context.Context, c *cmd.ChangeUserEmail) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		cmd := "UPDATE users SET email = $3, email_supressed_at = NULL WHERE id = $1 AND tenant_id = $2"
		_, err := trx.Execute(cmd, c.UserID, tenant.ID, strings.ToLower(c.Email))
		if err != nil {
			return errors.Wrap(err, "failed to update user's email")
		}
		return nil
	})
}

func updateCurrentUserSettings(ctx context.Context, c *cmd.UpdateCurrentUserSettings) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if user != nil && c.Settings != nil && len(c.Settings) > 0 {
			query := `
			INSERT INTO user_settings (tenant_id, user_id, key, value)
			VALUES ($1, $2, $3, $4) ON CONFLICT (user_id, key) DO UPDATE SET value = $4
			`

			for key, value := range c.Settings {
				_, err := trx.Execute(query, tenant.ID, user.ID, key, value)
				if err != nil {
					return errors.Wrap(err, "failed to update user settings")
				}
			}
		}

		return nil
	})
}

func getCurrentUserSettings(ctx context.Context, q *query.GetCurrentUserSettings) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		q.Result = make(map[string]string)

		var settings []*dbUserSetting
		err := trx.Select(&settings, "SELECT key, value FROM user_settings WHERE user_id = $1 AND tenant_id = $2", user.ID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get user settings")
		}

		for _, e := range enum.AllNotificationEvents {
			for _, r := range e.DefaultEnabledUserRoles {
				if r == user.Role {
					q.Result[e.UserSettingsKeyName] = e.DefaultSettingValue
				}
			}
		}

		for _, s := range settings {
			q.Result[s.Key] = s.Value
		}

		return nil
	})
}

func registerUser(ctx context.Context, c *cmd.RegisterUser) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, _ *entity.User) error {
		now := time.Now()
		c.User.Status = enum.UserActive
		c.User.Email = strings.ToLower(strings.TrimSpace(c.User.Email))
		if err := trx.Get(&c.User.ID,
			"INSERT INTO users (name, email, created_at, tenant_id, role, status, avatar_type, avatar_bkey) VALUES ($1, $2, $3, $4, $5, $6, $7, '') RETURNING id",
			c.User.Name, c.User.Email, now, tenant.ID, c.User.Role, enum.UserActive, enum.AvatarTypeGravatar); err != nil {
			return errors.Wrap(err, "failed to register new user")
		}

		for _, provider := range c.User.Providers {
			cmd := "INSERT INTO user_providers (tenant_id, user_id, provider, provider_uid, created_at) VALUES ($1, $2, $3, $4, $5)"
			if _, err := trx.Execute(cmd, tenant.ID, c.User.ID, provider.Name, provider.UID, now); err != nil {
				return errors.Wrap(err, "failed to add provider to new user")
			}
		}

		return nil
	})
}

func registerUserProvider(ctx context.Context, c *cmd.RegisterUserProvider) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		cmd := "INSERT INTO user_providers (tenant_id, user_id, provider, provider_uid, created_at) VALUES ($1, $2, $3, $4, $5)"
		_, err := trx.Execute(cmd, tenant.ID, c.UserID, c.ProviderName, c.ProviderUID, time.Now())
		if err != nil {
			return errors.Wrap(err, "failed to add provider '%s:%s' to user with id '%d'", c.ProviderName, c.ProviderUID, c.UserID)
		}
		return nil
	})
}

func updateCurrentUser(ctx context.Context, c *cmd.UpdateCurrentUser) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if c.Avatar.Remove {
			c.Avatar.BlobKey = ""
		}

		if c.Name == "" {
			cmd := "UPDATE users SET avatar_type = $3, avatar_bkey = $4 WHERE id = $1 AND tenant_id = $2"
			_, err := trx.Execute(cmd, user.ID, tenant.ID, c.AvatarType, c.Avatar.BlobKey)
			if err != nil {
				return errors.Wrap(err, "failed to update user avatar")
			}
			return nil
		}

		cmd := "UPDATE users SET name = $3, avatar_type = $4, avatar_bkey = $5 WHERE id = $1 AND tenant_id = $2"
		_, err := trx.Execute(cmd, user.ID, tenant.ID, c.Name, c.AvatarType, c.Avatar.BlobKey)
		if err != nil {
			return errors.Wrap(err, "failed to update user")
		}
		return nil
	})
}

func getUserByID(ctx context.Context, q *query.GetUserByID) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		u, err := queryUser(ctx, trx, "id = $1 AND tenant_id = $2", q.UserID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get user with id '%d'", q.UserID)
		}
		q.Result = u
		return nil
	})
}

func getUserByEmail(ctx context.Context, q *query.GetUserByEmail) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		email := strings.ToLower(q.Email)
		u, err := queryUser(ctx, trx, "email = $1 AND tenant_id = $2", email, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get user with email '%s'", email)
		}
		q.Result = u
		return nil
	})
}

func getUserByProvider(ctx context.Context, q *query.GetUserByProvider) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var userID int
		if err := trx.Scalar(&userID, `
			SELECT user_id 
			FROM user_providers up 
			INNER JOIN users u 
			ON u.id = up.user_id 
			AND u.tenant_id = up.tenant_id 
			WHERE up.provider = $1 
			AND up.provider_uid = $2 
			AND u.tenant_id = $3`, q.Provider, q.UID, tenant.ID); err != nil {
			return errors.Wrap(err, "failed to get user by provider '%s' and uid '%s'", q.Provider, q.UID)
		}

		byID := &query.GetUserByID{UserID: userID}
		err := getUserByID(ctx, byID)
		q.Result = byID.Result
		return err
	})
}

func getAllUserProviders(ctx context.Context, q *query.GetAllUserProviders) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		err := trx.Select(&q.Result, `
            SELECT user_id, provider, provider_uid
            FROM user_providers
            WHERE tenant_id = $1
        `, tenant.ID)

		if err != nil {
			return errors.Wrap(err, "failed to get all user providers")
		}

		return nil
	})
}

func getAllUsers(ctx context.Context, q *query.GetAllUsers) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var users []*dbUser
		err := trx.Select(&users, `
			SELECT id, name, email, tenant_id, role, status, avatar_type, avatar_bkey, visual_role
			FROM users 
			WHERE tenant_id = $1 
			AND status != $2
			ORDER BY id`, tenant.ID, enum.UserDeleted)
		if err != nil {
			return errors.Wrap(err, "failed to get all users")
		}

		q.Result = make([]*entity.User, len(users))
		for i, user := range users {
			q.Result[i] = user.toModel(ctx)
		}
		return nil
	})
}

func getAllUsersNames(ctx context.Context, q *query.GetAllUsersNames) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var users []*dbUser
		query := `
			SELECT id, name
			FROM users 
			WHERE tenant_id = $1 
			AND status != $2`

		args := []any{tenant.ID, enum.UserDeleted}

		if len(q.Query) < 2 || len(q.Query) > 10 {
			q.Result = []*dto.UserNames{}
			return nil
		}

		if q.Query != "" {
			query += ` AND LOWER(name) LIKE LOWER($3)`
			args = append(args, "%"+q.Query+"%")
		}

		query += ` ORDER BY id`

		limit := 10
		if q.Limit > 0 {
			if q.Limit < limit {
				limit = q.Limit
			}
		}

		query += ` LIMIT $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, limit)

		err := trx.Select(&users, query, args...)
		if err != nil {
			return errors.Wrap(err, "failed to get filtered users")
		}

		q.Result = make([]*dto.UserNames, len(users))
		for i, user := range users {
			q.Result[i] = &dto.UserNames{
				ID:   int(user.ID.Int64),
				Name: user.Name.String,
			}
		}
		return nil
	})
}

func getUsersByIDs(ctx context.Context, q *query.GetUsersByIDs) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if len(q.UserIDs) == 0 {
			q.Result = []*entity.User{}
			return nil
		}

		var users []*dbUser
		placeholders := make([]string, len(q.UserIDs))
		args := make([]interface{}, len(q.UserIDs)+2)
		args[0] = tenant.ID
		args[1] = enum.UserDeleted
		for i, id := range q.UserIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+3)
			args[i+2] = id
		}

		query := fmt.Sprintf(`
			SELECT id, name, email, tenant_id, role, status, avatar_type, avatar_bkey, visual_role
			FROM users 
			WHERE tenant_id = $1 
			AND status != $2
			AND id IN (%s)
			ORDER BY id`, strings.Join(placeholders, ","))

		err := trx.Select(&users, query, args...)
		if err != nil {
			return errors.Wrap(err, "failed to get users by ids")
		}

		q.Result = make([]*entity.User, len(users))
		for i, user := range users {
			q.Result[i] = user.toModel(ctx)
		}
		return nil
	})
}

func queryUser(ctx context.Context, trx *dbx.Trx, filter string, args ...any) (*entity.User, error) {
	user := dbUser{}
	sql := fmt.Sprintf("SELECT id, name, email, tenant_id, role, visual_role, status, avatar_type, avatar_bkey FROM users WHERE status != %d AND ", enum.UserDeleted)
	err := trx.Get(&user, sql+filter, args...)
	if err != nil {
		return nil, err
	}

	err = trx.Select(&user.Providers, "SELECT provider_uid, provider FROM user_providers WHERE user_id = $1", user.ID.Int64)
	if err != nil {
		return nil, err
	}

	return user.toModel(ctx), nil
}

func getUserProfileStats(ctx context.Context, q *query.GetUserProfileStats) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var postCount int
		err := trx.Scalar(&postCount, `
			SELECT COUNT(*) 
			FROM posts 
			WHERE user_id = $1 
			AND tenant_id = $2 
			AND status != $3
		`, q.UserID, tenant.ID, enum.PostDeleted)
		if err != nil {
			return errors.Wrap(err, "failed to count user posts")
		}
		q.Result.Posts = postCount

		var commentCount int
		err = trx.Scalar(&commentCount, `
			SELECT COUNT(*) 
			FROM comments 
			WHERE user_id = $1 
			AND tenant_id = $2 
			AND deleted_at IS NULL
		`, q.UserID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to count user comments")
		}
		q.Result.Comments = commentCount

		var voteCount int
		err = trx.Scalar(&voteCount, "SELECT COUNT(*) FROM post_votes WHERE user_id = $1 AND tenant_id = $2", q.UserID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to count user votes")
		}
		q.Result.Votes = voteCount

		return nil
	})
}

func getUserProfileStanding(ctx context.Context, q *query.GetUserProfileStanding) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		// Get warnings
		var warnings []*dbUserWarning
		err := trx.Select(&warnings, `
			SELECT id, reason, created_at, expires_at 
			FROM user_warnings 
			WHERE user_id = $1 AND tenant_id = $2
			ORDER BY created_at DESC
		`, q.UserID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get user warnings")
		}

		q.Result.Warnings = make([]struct {
			ID        int        `json:"id"`
			Reason    string     `json:"reason"`
			CreatedAt time.Time  `json:"createdAt"`
			ExpiresAt *time.Time `json:"expiresAt,omitempty"`
		}, len(warnings))

		for i, w := range warnings {
			var expiresAt *time.Time
			if w.ExpiresAt.Valid {
				expiresAt = &w.ExpiresAt.Time
			}
			q.Result.Warnings[i] = struct {
				ID        int        `json:"id"`
				Reason    string     `json:"reason"`
				CreatedAt time.Time  `json:"createdAt"`
				ExpiresAt *time.Time `json:"expiresAt,omitempty"`
			}{
				ID:        w.ID,
				Reason:    w.Reason,
				CreatedAt: w.CreatedAt,
				ExpiresAt: expiresAt,
			}
		}

		// Get mutes
		var mutes []*dbUserMute
		err = trx.Select(&mutes, `
			SELECT id, reason, created_at, expires_at 
			FROM user_mutes 
			WHERE user_id = $1 AND tenant_id = $2
			ORDER BY created_at DESC
		`, q.UserID, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get user mutes")
		}

		q.Result.Mutes = make([]struct {
			ID        int        `json:"id"`
			Reason    string     `json:"reason"`
			CreatedAt time.Time  `json:"createdAt"`
			ExpiresAt *time.Time `json:"expiresAt,omitempty"`
		}, len(mutes))

		for i, m := range mutes {
			var expiresAt *time.Time
			if m.ExpiresAt.Valid {
				expiresAt = &m.ExpiresAt.Time
			}
			q.Result.Mutes[i] = struct {
				ID        int        `json:"id"`
				Reason    string     `json:"reason"`
				CreatedAt time.Time  `json:"createdAt"`
				ExpiresAt *time.Time `json:"expiresAt,omitempty"`
			}{
				ID:        m.ID,
				Reason:    m.Reason,
				CreatedAt: m.CreatedAt,
				ExpiresAt: expiresAt,
			}
		}

		return nil
	})
}

func searchUserContent(ctx context.Context, q *query.SearchUserContent) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if q.Limit <= 0 || q.Limit > 10 {
			q.Limit = 10
		}

		if q.Offset < 0 {
			q.Offset = 0
		}

		if q.SortBy == "" {
			q.SortBy = "createdAt"
		}

		if q.SortOrder == "" {
			q.SortOrder = "desc"
		}

		dbSortField := "created_at"
		if q.SortBy == "title" {
			dbSortField = "title"
		}

		dbSortOrder := "DESC"
		if q.SortOrder == "asc" {
			dbSortOrder = "ASC"
		}

		q.Result.Posts = []query.UserPostResult{}
		q.Result.Comments = []query.UserCommentResult{}

		if q.ContentType == "all" || q.ContentType == "posts" || q.ContentType == "" {
			// Search posts
			var posts []*dbUserPost
			postQuery := `
				SELECT number as id, title, created_at 
				FROM posts 
				WHERE user_id = $1 
				AND tenant_id = $2
				AND status != $3
			`
			args := []any{q.UserID, tenant.ID, enum.PostDeleted}

			if q.Query != "" {
				postQuery += " AND title ILIKE $4"
				args = append(args, "%"+q.Query+"%")
			}

			postQuery += fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d",
				dbSortField, dbSortOrder, len(args)+1, len(args)+2)
			args = append(args, q.Limit, q.Offset)

			err := trx.Select(&posts, postQuery, args...)
			if err != nil {
				return errors.Wrap(err, "failed to search user posts")
			}

			q.Result.Posts = make([]query.UserPostResult, len(posts))
			for i, p := range posts {
				q.Result.Posts[i] = query.UserPostResult{
					ID:        p.ID,
					Title:     p.Title,
					CreatedAt: p.CreatedAt,
				}
			}
		}

		if q.ContentType == "all" || q.ContentType == "comments" || q.ContentType == "" {
			// Search comments
			var comments []*dbUserComment
			commentQuery := `
				SELECT c.id, c.content, c.post_id, p.number as post_number, p.title as post_title, c.created_at 
				FROM comments c
				JOIN posts p ON p.id = c.post_id AND p.status != $4
				WHERE c.user_id = $1 AND c.tenant_id = $2 AND c.deleted_at IS NULL
			`
			args := []any{q.UserID, tenant.ID, "%" + q.Query + "%", enum.PostDeleted}

			if q.Query != "" {
				commentQuery += " AND c.content ILIKE $3"
			} else {
				commentQuery = strings.Replace(commentQuery, "AND p.status != $4", "AND p.status != $3", 1)
				args = []any{q.UserID, tenant.ID, enum.PostDeleted}
			}

			commentSortField := "c.created_at"
			if q.SortBy == "title" {
				commentSortField = "p.title"
			}

			commentQuery += fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d",
				commentSortField, dbSortOrder, len(args)+1, len(args)+2)
			args = append(args, q.Limit, q.Offset)

			err := trx.Select(&comments, commentQuery, args...)
			if err != nil {
				return errors.Wrap(err, "failed to search user comments")
			}

			q.Result.Comments = make([]query.UserCommentResult, len(comments))
			for i, c := range comments {
				q.Result.Comments[i] = query.UserCommentResult{
					ID:         c.ID,
					Content:    c.Content,
					PostNumber: c.PostNumber,
					PostTitle:  c.PostTitle,
					CreatedAt:  c.CreatedAt,
				}
			}
		}

		if q.ContentType == "voted" {
			var votedPosts []*dbUserPost
			votedQuery := `
				SELECT p.number as id, p.title, p.created_at 
				FROM posts p
				JOIN post_votes pv ON p.id = pv.post_id
				WHERE pv.user_id = $1 
				AND p.tenant_id = $2 
				AND p.status != $3
			`
			args := []any{q.UserID, tenant.ID, enum.PostDeleted}

			if q.VoteType != 0 {
				votedQuery += " AND pv.vote_type = $4"
				args = append(args, q.VoteType)
			}

			if q.Query != "" {
				paramIndex := len(args) + 1
				votedQuery += fmt.Sprintf(" AND p.title ILIKE $%d", paramIndex)
				args = append(args, "%"+q.Query+"%")
			}

			votedQuery += fmt.Sprintf(" ORDER BY p.%s %s LIMIT $%d OFFSET $%d",
				dbSortField, dbSortOrder, len(args)+1, len(args)+2)
			args = append(args, q.Limit, q.Offset)

			err := trx.Select(&votedPosts, votedQuery, args...)
			if err != nil {
				return errors.Wrap(err, "failed to get user voted posts")
			}

			q.Result.Posts = make([]query.UserPostResult, len(votedPosts))
			for i, p := range votedPosts {
				q.Result.Posts[i] = query.UserPostResult{
					ID:        p.ID,
					Title:     p.Title,
					CreatedAt: p.CreatedAt,
				}
			}
		}

		return nil
	})
}

func muteUser(ctx context.Context, c *cmd.MuteUser) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		// Ensure we have a valid expiration time
		if c.ExpiresAt.IsZero() {
			c.ExpiresAt = time.Now().Add(24 * time.Hour) // Default to 24 hours if not specified
		}

		// First, expire any existing active mutes
		_, err := trx.Execute(`
			UPDATE user_mutes 
			SET expires_at = NOW() 
			WHERE user_id = $1 AND tenant_id = $2 AND (expires_at IS NULL OR expires_at > NOW())
		`, c.UserID, tenant.ID)
		if err != nil {
			return err
		}

		// Then insert the new mute
		_, err = trx.Execute(`
			INSERT INTO user_mutes (user_id, tenant_id, reason, created_at, expires_at, created_by)
			VALUES ($1, $2, $3, NOW(), $4, $5)
		`, c.UserID, tenant.ID, c.Reason, c.ExpiresAt, user.ID)
		return err
	})
}

func warnUser(ctx context.Context, c *cmd.WarnUser) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var expiresAt sql.NullTime
		if c.ExpiresAt.IsZero() {
			// If no expiration time is set, keep it as NULL
			// I don't know when this will be used, but it takes nothing to add it
			expiresAt.Valid = false
		} else {
			// If expiration time is set, use it
			expiresAt.Valid = true
			expiresAt.Time = c.ExpiresAt
		}

		_, err := trx.Execute(`
			INSERT INTO user_warnings (user_id, tenant_id, reason, created_at, expires_at, created_by)
			VALUES ($1, $2, $3, NOW(), $4, $5)
		`, c.UserID, tenant.ID, c.Reason, expiresAt, user.ID)
		return err
	})
}

func updateUserAvatar(ctx context.Context, c *cmd.UpdateUserAvatar) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, _ *entity.User) error {
		if c.Avatar.Remove {
			c.Avatar.BlobKey = ""
		}
		cmd := "UPDATE users SET avatar_type = $3, avatar_bkey = $4 WHERE id = $1 AND tenant_id = $2"
		_, err := trx.Execute(cmd, c.UserID, tenant.ID, c.AvatarType, c.Avatar.BlobKey)
		if err != nil {
			return errors.Wrap(err, "failed to update user avatar")
		}
		return nil
	})
}

func updateUser(ctx context.Context, c *cmd.UpdateUser) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, _ *entity.User) error {
		cmd := "UPDATE users SET name = $3 WHERE id = $1 AND tenant_id = $2"
		_, err := trx.Execute(cmd, c.UserID, tenant.ID, c.Name)
		if err != nil {
			return errors.Wrap(err, "failed to update user")
		}
		return nil
	})
}
