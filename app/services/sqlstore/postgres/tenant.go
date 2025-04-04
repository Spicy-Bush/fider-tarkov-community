package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/dbx"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
)

type dbTenant struct {
	ID                 int            `db:"id"`
	Name               string         `db:"name"`
	Subdomain          string         `db:"subdomain"`
	CNAME              string         `db:"cname"`
	Invitation         string         `db:"invitation"`
	WelcomeMessage     string         `db:"welcome_message"`
	Status             int            `db:"status"`
	Locale             string         `db:"locale"`
	IsPrivate          bool           `db:"is_private"`
	LogoBlobKey        string         `db:"logo_bkey"`
	CustomCSS          string         `db:"custom_css"`
	ProfanityWords     string         `db:"profanity_words"`
	IsEmailAuthAllowed bool           `db:"is_email_auth_allowed"`
	GeneralSettings    dbx.NullString `db:"general_settings"`
	MessageBanner      string         `db:"message_banner"`
}

func (t *dbTenant) toModel() *entity.Tenant {
	if t == nil {
		return nil
	}

	tenant := &entity.Tenant{
		ID:                 t.ID,
		Name:               t.Name,
		Subdomain:          t.Subdomain,
		CNAME:              t.CNAME,
		Invitation:         t.Invitation,
		WelcomeMessage:     t.WelcomeMessage,
		Status:             enum.TenantStatus(t.Status),
		Locale:             t.Locale,
		IsPrivate:          t.IsPrivate,
		LogoBlobKey:        t.LogoBlobKey,
		CustomCSS:          t.CustomCSS,
		ProfanityWords:     t.ProfanityWords,
		IsEmailAuthAllowed: t.IsEmailAuthAllowed,
		GeneralSettings:    &entity.GeneralSettings{},
		MessageBanner:      t.MessageBanner,
	}

	if t.GeneralSettings.Valid {
		var settings entity.GeneralSettings
		if err := json.Unmarshal([]byte(t.GeneralSettings.String), &settings); err == nil {
			tenant.GeneralSettings = &settings
		}
	}

	return tenant
}

type dbEmailVerification struct {
	ID         int                        `db:"id"`
	Name       string                     `db:"name"`
	Email      string                     `db:"email"`
	Key        string                     `db:"key"`
	Kind       enum.EmailVerificationKind `db:"kind"`
	UserID     dbx.NullInt                `db:"user_id"`
	CreatedAt  time.Time                  `db:"created_at"`
	ExpiresAt  time.Time                  `db:"expires_at"`
	VerifiedAt dbx.NullTime               `db:"verified_at"`
}

func (t *dbEmailVerification) toModel() *entity.EmailVerification {
	model := &entity.EmailVerification{
		Name:       t.Name,
		Email:      t.Email,
		Key:        t.Key,
		Kind:       t.Kind,
		CreatedAt:  t.CreatedAt,
		ExpiresAt:  t.ExpiresAt,
		VerifiedAt: nil,
	}

	if t.VerifiedAt.Valid {
		model.VerifiedAt = &t.VerifiedAt.Time
	}

	if t.UserID.Valid {
		model.UserID = int(t.UserID.Int64)
	}

	return model
}

func updateGeneralSettings(ctx context.Context, c *cmd.UpdateGeneralSettings) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if c.Settings == nil {
			c.Settings = &entity.GeneralSettings{
				PostLimits:                 make(map[string]entity.PostLimit),
				CommentLimits:              make(map[string]entity.CommentLimit),
				TitleLengthMin:             10,
				TitleLengthMax:             100,
				DescriptionLengthMin:       10,
				DescriptionLengthMax:       1000,
				MaxImagesPerPost:           3,
				MaxImagesPerComment:        2,
				PostingDisabledFor:         []string{},
				CommentingDisabledFor:      []string{},
				PostingGloballyDisabled:    false,
				CommentingGloballyDisabled: false,
			}
		}

		settingsJSON, err := json.Marshal(c.Settings)
		if err != nil {
			return errors.Wrap(err, "failed to marshal general settings")
		}

		query := "UPDATE tenants SET general_settings = $1 WHERE id = $2"
		_, err = trx.Execute(query, settingsJSON, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to update general settings")
		}

		tenant.GeneralSettings = c.Settings
		return nil
	})
}

func isCNAMEAvailable(ctx context.Context, q *query.IsCNAMEAvailable) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		tenantID := 0
		if tenant != nil {
			tenantID = tenant.ID
		}

		exists, err := trx.Exists("SELECT id FROM tenants WHERE cname = $1 AND id <> $2", q.CNAME, tenantID)
		if err != nil {
			q.Result = false
			return errors.Wrap(err, "failed to check if tenant exists with CNAME '%s'", q.CNAME)
		}
		q.Result = !exists
		return nil
	})
}

func isSubdomainAvailable(ctx context.Context, q *query.IsSubdomainAvailable) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		exists, err := trx.Exists("SELECT id FROM tenants WHERE subdomain = $1", q.Subdomain)
		if err != nil {
			q.Result = false
			return errors.Wrap(err, "failed to check if tenant exists with subdomain '%s'", q.Subdomain)
		}
		q.Result = !exists
		return nil
	})
}

func updateTenantPrivacySettings(ctx context.Context, c *cmd.UpdateTenantPrivacySettings) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute("UPDATE tenants SET is_private = $1 WHERE id = $2", c.IsPrivate, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed update tenant privacy settings")
		}
		return nil
	})
}

func updateTenantEmailAuthAllowedSettings(ctx context.Context, c *cmd.UpdateTenantEmailAuthAllowedSettings) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		_, err := trx.Execute("UPDATE tenants SET is_email_auth_allowed = $1 WHERE id = $2", c.IsEmailAuthAllowed, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed update tenant allowing email auth settings")
		}
		return nil
	})
}

func updateTenantSettings(ctx context.Context, c *cmd.UpdateTenantSettings) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		if c.Logo.Remove {
			c.Logo.BlobKey = ""
		}

		query := "UPDATE tenants SET name = $1, invitation = $2, welcome_message = $3, cname = $4, logo_bkey = $5, locale = $6 WHERE id = $7"
		_, err := trx.Execute(query, c.Title, c.Invitation, c.WelcomeMessage, c.CNAME, c.Logo.BlobKey, c.Locale, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed update tenant settings")
		}

		tenant.Name = c.Title
		tenant.Invitation = c.Invitation
		tenant.CNAME = c.CNAME
		tenant.WelcomeMessage = c.WelcomeMessage

		return nil
	})
}

func updateTenantAdvancedSettings(ctx context.Context, c *cmd.UpdateTenantAdvancedSettings) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		// Convert newline-separated text to comma-separated.
		profanity := strings.TrimSpace(c.ProfanityWords)
		if profanity != "" {
			profanity = strings.ReplaceAll(profanity, "\n", ",")
		}

		query := "UPDATE tenants SET custom_css = $1, profanity_words = $2 WHERE id = $3"
		_, err := trx.Execute(query, c.CustomCSS, profanity, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to update tenant advanced settings")
		}

		tenant.CustomCSS = c.CustomCSS
		tenant.ProfanityWords = profanity
		return nil
	})
}

func activateTenant(ctx context.Context, c *cmd.ActivateTenant) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		query := "UPDATE tenants SET status = $1 WHERE id = $2"
		_, err := trx.Execute(query, enum.TenantActive, c.TenantID)
		if err != nil {
			return errors.Wrap(err, "failed to activate tenant with id '%d'", c.TenantID)
		}
		return nil
	})
}

func getVerificationByKey(ctx context.Context, q *query.GetVerificationByKey) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		verification := dbEmailVerification{}

		query := "SELECT id, email, name, key, created_at, verified_at, expires_at, kind, user_id FROM email_verifications WHERE key = $1 AND kind = $2 LIMIT 1"
		err := trx.Get(&verification, query, q.Key, q.Kind)
		if err != nil {
			return errors.Wrap(err, "failed to get email verification by its key")
		}

		q.Result = verification.toModel()
		return nil
	})
}

func saveVerificationKey(ctx context.Context, c *cmd.SaveVerificationKey) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var userID any
		if c.Request.GetUser() != nil {
			userID = c.Request.GetUser().ID
		}

		query := "INSERT INTO email_verifications (tenant_id, email, created_at, expires_at, key, name, kind, user_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)"
		_, err := trx.Execute(query, tenant.ID, c.Request.GetEmail(), time.Now(), time.Now().Add(c.Duration), c.Key, c.Request.GetName(), c.Request.GetKind(), userID)
		if err != nil {
			return errors.Wrap(err, "failed to save verification key for kind '%d'", c.Request.GetKind())
		}
		return nil
	})
}

func UpdateMessageBanner(ctx context.Context, c *cmd.UpdateMessageBanner) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		query := "UPDATE tenants SET message_banner = $1 WHERE id = $2"
		_, err := trx.Execute(query, c.MessageBanner, tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to update message banner")
		}
		return nil
	})
}

func setKeyAsVerified(ctx context.Context, c *cmd.SetKeyAsVerified) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		query := "UPDATE email_verifications SET verified_at = $1 WHERE tenant_id = $2 AND key = $3 AND verified_at IS NULL"
		_, err := trx.Execute(query, time.Now(), tenant.ID, c.Key)
		if err != nil {
			return errors.Wrap(err, "failed to update verified date of email verification request")
		}
		return nil
	})
}

func createTenant(ctx context.Context, c *cmd.CreateTenant) error {
	return using(ctx, func(trx *dbx.Trx, _ *entity.Tenant, _ *entity.User) error {
		now := time.Now()

		var id int
		err := trx.Get(&id,
			`INSERT INTO tenants (name, subdomain, created_at, cname, invitation, welcome_message, status, is_private, custom_css, logo_bkey, locale, is_email_auth_allowed, profanity_words) 
			 VALUES ($1, $2, $3, '', '', '', $4, false, '', '', $5, true, '') 
			 RETURNING id`, c.Name, c.Subdomain, now, c.Status, env.Config.Locale)
		if err != nil {
			return err
		}

		if env.IsBillingEnabled() {
			trialEndsAt := time.Now().AddDate(0, 0, 15) // 15 days
			_, err := trx.Execute(
				`INSERT INTO tenants_billing (tenant_id, trial_ends_at, status, paddle_subscription_id, paddle_plan_id) 
				 VALUES ($1, $2, $3, '', '')`, id, trialEndsAt, enum.BillingTrial)
			if err != nil {
				return err
			}
		}

		byDomain := &query.GetTenantByDomain{Domain: c.Subdomain}
		err = bus.Dispatch(ctx, byDomain)
		c.Result = byDomain.Result
		return err
	})
}

func getFirstTenant(ctx context.Context, q *query.GetFirstTenant) error {
	return using(ctx, func(trx *dbx.Trx, _ *entity.Tenant, _ *entity.User) error {
		tenant := dbTenant{}

		err := trx.Get(&tenant, `
			SELECT id, name, subdomain, cname, invitation, locale, welcome_message, status, is_private, logo_bkey, custom_css, is_email_auth_allowed, profanity_words, general_settings, message_banner
			FROM tenants
			ORDER BY id LIMIT 1
		`)

		if err != nil {
			return errors.Wrap(err, "failed to get first tenant")
		}

		q.Result = tenant.toModel()
		return nil
	})
}

func getTenantByDomain(ctx context.Context, q *query.GetTenantByDomain) error {
	return using(ctx, func(trx *dbx.Trx, _ *entity.Tenant, _ *entity.User) error {
		tenant := dbTenant{}

		err := trx.Get(&tenant, `
			SELECT id, name, subdomain, cname, invitation, locale, welcome_message, status, is_private, logo_bkey, custom_css, is_email_auth_allowed, profanity_words, general_settings, message_banner
			FROM tenants t
			WHERE subdomain = $1 OR subdomain = $2 OR cname = $3 
			ORDER BY cname DESC
		`, env.Subdomain(q.Domain), q.Domain, q.Domain)
		if err != nil {
			return errors.Wrap(err, "failed to get tenant with domain '%s'", q.Domain)
		}

		q.Result = tenant.toModel()
		return nil
	})
}

func getTenantProfanityWords(ctx context.Context, q *query.GetTenantProfanityWords) error {
	return using(ctx, func(trx *dbx.Trx, tenant *entity.Tenant, user *entity.User) error {
		var profanityWords string
		err := trx.Get(&profanityWords, "SELECT profanity_words FROM tenants WHERE id = $1", tenant.ID)
		if err != nil {
			return errors.Wrap(err, "failed to get profanity words")
		}

		q.Result = profanityWords
		return nil
	})
}
