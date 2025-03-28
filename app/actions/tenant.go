package actions

import (
	"context"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/dto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/jwt"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

// CreateTenant is the input model used to create a tenant
type CreateTenant struct {
	Token           string `json:"token"`
	Name            string `json:"name"`
	Email           string `json:"email" format:"lower"`
	VerificationKey string
	TenantName      string `json:"tenantName"`
	LegalAgreement  bool   `json:"legalAgreement"`
	Subdomain       string `json:"subdomain" format:"lower"`
	UserClaims      *jwt.OAuthClaims
}

func NewCreateTenant() *CreateTenant {
	return &CreateTenant{
		VerificationKey: entity.GenerateEmailVerificationKey(),
	}
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *CreateTenant) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return true
}

// Validate if current model is valid
func (action *CreateTenant) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	var err error
	if action.Name == "" && action.Email == "" {
		if action.Token == "" {
			result.AddFieldFailure("token", "Please identify yourself before proceeding.")
		} else {
			if action.UserClaims, err = jwt.DecodeOAuthClaims(action.Token); err != nil {
				return validate.Error(err)
			}
		}
	} else {
		if action.Email == "" {
			result.AddFieldFailure("email", "Email is required.")
		} else {
			messages := validate.Email(ctx, action.Email)
			result.AddFieldFailure("email", messages...)
		}

		if action.Name == "" {
			result.AddFieldFailure("name", "Name is required.")
		}
		if len(action.Name) > 60 {
			result.AddFieldFailure("name", "Name must have less than 60 characters.")
		}
	}

	if env.IsSingleHostMode() {
		action.Subdomain = "default"
	}

	if action.TenantName == "" {
		result.AddFieldFailure("tenantName", "Name is required.")
	}

	messages, err := validate.Subdomain(ctx, action.Subdomain)
	if err != nil {
		return validate.Error(err)
	}

	result.AddFieldFailure("subdomain", messages...)

	if env.HasLegal() && !action.LegalAgreement {
		result.AddFieldFailure("legalAgreement", "You must agree before proceeding.")
	}

	return result
}

// GetEmail returns the email being verified
func (action *CreateTenant) GetEmail() string {
	return action.Email
}

// GetName returns the name of the email owner
func (action *CreateTenant) GetName() string {
	return action.Name
}

// GetUser returns the current user performing this action
func (action *CreateTenant) GetUser() *entity.User {
	return nil
}

// GetKind returns EmailVerificationKindSignUp
func (action *CreateTenant) GetKind() enum.EmailVerificationKind {
	return enum.EmailVerificationKindSignUp
}

// UpdateTenantSettings is the input model used to update tenant settings
type UpdateTenantSettings struct {
	Logo           *dto.ImageUpload `json:"logo"`
	Title          string           `json:"title"`
	Invitation     string           `json:"invitation"`
	WelcomeMessage string           `json:"welcomeMessage"`
	Locale         string           `json:"locale"`
	CNAME          string           `json:"cname" format:"lower"`
}

func NewUpdateTenantSettings() *UpdateTenantSettings {
	return &UpdateTenantSettings{
		Logo: &dto.ImageUpload{},
	}
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *UpdateTenantSettings) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.Role == enum.RoleAdministrator
}

// Validate if current model is valid
func (action *UpdateTenantSettings) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	tenant, hasTenant := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	if hasTenant {
		action.Logo.BlobKey = tenant.LogoBlobKey
	}

	messages, err := validate.ImageUpload(ctx, action.Logo, validate.ImageUploadOpts{
		IsRequired:   false,
		MinHeight:    200,
		MinWidth:     200,
		MaxKilobytes: 5000,
		ExactRatio:   true,
	})
	if err != nil {
		return validate.Error(err)
	}
	result.AddFieldFailure("logo", messages...)

	if action.Title == "" {
		result.AddFieldFailure("title", "Title is required.")
	}

	if len(action.Title) > 60 {
		result.AddFieldFailure("title", "Title must have less than 60 characters.")
	}

	if len(action.Invitation) > 60 {
		result.AddFieldFailure("invitation", "Invitation must have less than 60 characters.")
	}

	if !i18n.IsValidLocale(action.Locale) {
		result.AddFieldFailure("locale", "Locale is invalid.")
	}

	if action.CNAME != "" {
		messages := validate.CNAME(ctx, action.CNAME)
		result.AddFieldFailure("cname", messages...)
	}

	return result
}

// UpdateTenantAdvancedSettings is the input model used to update tenant advanced settings
type UpdateTenantAdvancedSettings struct {
	CustomCSS      string `json:"customCSS"`
	ProfanityWords string `json:"profanityWords"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *UpdateTenantAdvancedSettings) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.Role == enum.RoleAdministrator
}

// Validate if current model is valid
func (action *UpdateTenantAdvancedSettings) Validate(ctx context.Context, user *entity.User) *validate.Result {
	return validate.Success()
}

// UpdateTenantPrivacy is the input model used to update tenant privacy settings
type UpdateTenantPrivacy struct {
	IsPrivate bool `json:"isPrivate"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *UpdateTenantPrivacy) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.Role == enum.RoleAdministrator
}

// Validate if current model is valid
func (action *UpdateTenantPrivacy) Validate(ctx context.Context, user *entity.User) *validate.Result {
	return validate.Success()
}

// UpdateTenantEmailAuthAllowed is the input model used to update tenant privacy settings
type UpdateTenantEmailAuthAllowed struct {
	IsEmailAuthAllowed bool `json:"isEmailAuthAllowed"`
}

// IsAuthorized returns true if current user is authorized to perform this action
func (action *UpdateTenantEmailAuthAllowed) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.Role == enum.RoleAdministrator
}

// Validate if current model is valid
func (action *UpdateTenantEmailAuthAllowed) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	activeProviders := &query.ListActiveOAuthProviders{}
	if err := bus.Dispatch(ctx, activeProviders); err != nil {
		return validate.Failed("Cannot retrieve OAuth providers")
	}

	if len(activeProviders.Result) == 0 {
		result.AddFieldFailure("isEmailAuthAllowed", "You cannot disable email authentication without any other provider enabled.")
	}

	return result
}

type UpdateMessageBanner struct {
	MessageBanner string `json:"messageBanner"`
}

func (action *UpdateMessageBanner) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.Role == enum.RoleAdministrator || user.Role == enum.RoleCollaborator
}

func (action *UpdateMessageBanner) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	// DB has varchar(1000)
	if len(action.MessageBanner) > 1000 {
		result.AddFieldFailure("messageBanner", "Message banner must have less than 1000 characters.")
	}

	return result
}

type UpdateGeneralSettings struct {
	Settings *entity.GeneralSettings `json:"settings"`
}

func (action *UpdateGeneralSettings) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.IsAdministrator()
}

func (action *UpdateGeneralSettings) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	if action.Settings == nil {
		result.AddFieldFailure("settings", "Settings are required")
		return result
	}

	if action.Settings.TitleLengthMin < 1 {
		result.AddFieldFailure("settings.titleLengthMin", "Title minimum length must be at least 1")
	}
	if action.Settings.TitleLengthMax < action.Settings.TitleLengthMin {
		result.AddFieldFailure("settings.titleLengthMax", "Title maximum length must be greater than or equal to minimum length")
	}

	if action.Settings.DescriptionLengthMin < 1 {
		result.AddFieldFailure("settings.descriptionLengthMin", "Description minimum length must be at least 1")
	}
	if action.Settings.DescriptionLengthMax < action.Settings.DescriptionLengthMin {
		result.AddFieldFailure("settings.descriptionLengthMax", "Description maximum length must be greater than or equal to minimum length")
	}

	if action.Settings.MaxImagesPerPost < 0 {
		result.AddFieldFailure("settings.maxImagesPerPost", "Maximum images per post must be non-negative")
	}
	if action.Settings.MaxImagesPerComment < 0 {
		result.AddFieldFailure("settings.maxImagesPerComment", "Maximum images per comment must be non-negative")
	}

	for role, limit := range action.Settings.PostLimits {
		if limit.Count < 0 {
			result.AddFieldFailure("settings.postLimits."+role+".count", "Post count must be non-negative")
		}
		if limit.Hours < 1 && limit.Count > 0 {
			result.AddFieldFailure("settings.postLimits."+role+".hours", "Time period must be at least 1 hour")
		}
	}

	for role, limit := range action.Settings.CommentLimits {
		if limit.Count < 0 {
			result.AddFieldFailure("settings.commentLimits."+role+".count", "Comment count must be non-negative")
		}
		if limit.Hours < 1 && limit.Count > 0 {
			result.AddFieldFailure("settings.commentLimits."+role+".hours", "Time period must be at least 1 hour")
		}
	}

	return result
}
