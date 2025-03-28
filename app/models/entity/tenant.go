package entity

import "github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"

// Tenant represents a tenant
type Tenant struct {
	ID                 int               `json:"id"`
	Name               string            `json:"name"`
	Subdomain          string            `json:"subdomain"`
	Invitation         string            `json:"invitation"`
	WelcomeMessage     string            `json:"welcomeMessage"`
	CNAME              string            `json:"cname"`
	Status             enum.TenantStatus `json:"status"`
	Locale             string            `json:"locale"`
	IsPrivate          bool              `json:"isPrivate"`
	LogoBlobKey        string            `json:"logoBlobKey"`
	CustomCSS          string            `json:"-"`
	IsEmailAuthAllowed bool              `json:"isEmailAuthAllowed"`
	ProfanityWords     string            `json:"profanityWords"`
	GeneralSettings    *GeneralSettings  `json:"generalSettings"`
	MessageBanner      string            `json:"messageBanner"`
}

func (t *Tenant) IsDisabled() bool {
	return t.Status == enum.TenantDisabled
}

// TenantContact is a reference to an administrator account
type TenantContact struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Subdomain string `json:"subdomain"`
}

type PostLimit struct {
	Count int `json:"count"`
	Hours int `json:"hours"`
}
type CommentLimit struct {
	Count int `json:"count"`
	Hours int `json:"hours"`
}

type GeneralSettings struct {
	PostLimits                 map[string]PostLimit    `json:"postLimits"`
	CommentLimits              map[string]CommentLimit `json:"commentLimits"`
	TitleLengthMin             int                     `json:"titleLengthMin"`
	TitleLengthMax             int                     `json:"titleLengthMax"`
	DescriptionLengthMin       int                     `json:"descriptionLengthMin"`
	DescriptionLengthMax       int                     `json:"descriptionLengthMax"`
	MaxImagesPerPost           int                     `json:"maxImagesPerPost"`
	MaxImagesPerComment        int                     `json:"maxImagesPerComment"`
	PostingDisabledFor         []string                `json:"postingDisabledFor"`
	CommentingDisabledFor      []string                `json:"commentingDisabledFor"`
	PostingGloballyDisabled    bool                    `json:"postingGloballyDisabled"`
	CommentingGloballyDisabled bool                    `json:"commentingGloballyDisabled"`
}
