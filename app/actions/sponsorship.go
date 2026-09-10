package actions

import (
	"context"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

var allowedLocales = map[string]bool{"all": true, "en": true, "ru": true}

func isCollaboratorPlus(user *entity.User) bool {
	return user != nil && (user.IsAdministrator() || user.IsCollaborator())
}

type CreateSponsorshipPackage struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Slots        string `json:"slots"`
	DurationDays int    `json:"durationDays"`
	Sort         int    `json:"sort"`
}

func (a *CreateSponsorshipPackage) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return isCollaboratorPlus(user)
}

func (a *CreateSponsorshipPackage) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if strings.TrimSpace(a.Slug) == "" {
		result.AddFieldFailure("slug", "Slug is required")
	}
	if strings.TrimSpace(a.Name) == "" {
		result.AddFieldFailure("name", "Name is required")
	}
	if a.DurationDays <= 0 {
		result.AddFieldFailure("durationDays", "Duration must be positive")
	}
	return result
}

type UpdateSponsorshipPackage struct {
	ID           int    `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Slots        string `json:"slots"`
	DurationDays int    `json:"durationDays"`
	Sort         int    `json:"sort"`
}

func (a *UpdateSponsorshipPackage) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return isCollaboratorPlus(user)
}

func (a *UpdateSponsorshipPackage) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if a.ID <= 0 {
		result.AddFieldFailure("id", "Invalid ID")
	}
	if strings.TrimSpace(a.Slug) == "" {
		result.AddFieldFailure("slug", "Slug is required")
	}
	if strings.TrimSpace(a.Name) == "" {
		result.AddFieldFailure("name", "Name is required")
	}
	if a.DurationDays <= 0 {
		result.AddFieldFailure("durationDays", "Duration must be positive")
	}
	return result
}

type DeleteSponsorshipPackage struct {
	ID int `json:"id"`
}

func (a *DeleteSponsorshipPackage) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return isCollaboratorPlus(user)
}

func (a *DeleteSponsorshipPackage) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if a.ID <= 0 {
		result.AddFieldFailure("id", "Invalid ID")
	}
	return result
}

type createCampaignVersionJSON struct {
	ImageURL string `json:"imageUrl"`
	HTML     string `json:"html"`
	ClickURL string `json:"clickUrl"`
}

type createCampaignAssignmentJSON struct {
	PlacementID       string `json:"placementId"`
	CreativeVersionID int    `json:"creativeVersionId"`
}

type CreateSponsorshipCampaign struct {
	Name        string                         `json:"name"`
	Advertiser  string                         `json:"advertiser"`
	StartAt     time.Time                      `json:"startAt"`
	EndAt       time.Time                      `json:"endAt"`
	Weight      int                            `json:"weight"`
	Locale      string                         `json:"locale"`
	Enabled     bool                           `json:"enabled"`
	PackageID   *int                           `json:"packageId"`
	Version     *createCampaignVersionJSON     `json:"version"`
	Assignments []createCampaignAssignmentJSON `json:"assignments"`
}

func (a *CreateSponsorshipCampaign) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return isCollaboratorPlus(user)
}

func (a *CreateSponsorshipCampaign) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validateCampaignFields(validate.Success(), a.Name, a.Advertiser, a.StartAt, a.EndAt, a.Weight, a.Locale)
	if a.Locale == "" {
		a.Locale = "all"
	}
	if a.Version != nil {
		a.Version.ImageURL = strings.TrimSpace(a.Version.ImageURL)
		a.Version.HTML = strings.TrimSpace(a.Version.HTML)
		a.Version.ClickURL = strings.TrimSpace(a.Version.ClickURL)
		if a.Version.ClickURL == "" || !validate.IsHTTPOrHTTPSURL(a.Version.ClickURL) {
			result.AddFieldFailure("version.clickUrl", "clickUrl must be an http(s) URL")
		}
		if a.Version.ImageURL == "" && a.Version.HTML == "" {
			result.AddFieldFailure("version", "Provide imageUrl or html")
		}
	}
	if len(a.Assignments) > 0 && a.Version == nil {
		result.AddFieldFailure("assignments", "assignments require version on create")
	}
	seen := map[string]bool{}
	for _, asg := range a.Assignments {
		pid := strings.TrimSpace(asg.PlacementID)
		if pid == "" {
			result.AddFieldFailure("assignments", "placementId is required")
			break
		}
		if !entity.IsCatalogPlacementID(pid) {
			result.AddFieldFailure("assignments", "unknown placementId")
			break
		}
		if seen[pid] {
			result.AddFieldFailure("assignments", "duplicate placementId")
			break
		}
		seen[pid] = true
	}
	return result
}

type UpdateSponsorshipCampaign struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Advertiser    string    `json:"advertiser"`
	StartAt       time.Time `json:"startAt"`
	EndAt         time.Time `json:"endAt"`
	Weight        int       `json:"weight"`
	Locale        string    `json:"locale"`
	Enabled       bool      `json:"enabled"`
	PackageID     *int      `json:"packageId"`
	ConfigVersion int       `json:"configVersion"`
}

func (a *UpdateSponsorshipCampaign) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return isCollaboratorPlus(user)
}

func (a *UpdateSponsorshipCampaign) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if a.ID <= 0 {
		result.AddFieldFailure("id", "Invalid ID")
	}
	result = validateCampaignFields(result, a.Name, a.Advertiser, a.StartAt, a.EndAt, a.Weight, a.Locale)
	if a.Locale == "" {
		a.Locale = "all"
	}
	return result
}

type DeleteSponsorshipCampaign struct {
	ID int `json:"id"`
}

func (a *DeleteSponsorshipCampaign) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return isCollaboratorPlus(user)
}

func (a *DeleteSponsorshipCampaign) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if a.ID <= 0 {
		result.AddFieldFailure("id", "Invalid ID")
	}
	return result
}

func validateCampaignFields(result *validate.Result, name, advertiser string, startAt, endAt time.Time, weight int, locale string) *validate.Result {
	if strings.TrimSpace(name) == "" {
		result.AddFieldFailure("name", "Name is required")
	}
	if strings.TrimSpace(advertiser) == "" {
		result.AddFieldFailure("advertiser", "Advertiser / company name is required")
	}
	if startAt.IsZero() {
		result.AddFieldFailure("startAt", "Start date is required")
	}
	if endAt.IsZero() {
		result.AddFieldFailure("endAt", "End date is required")
	}
	if !startAt.IsZero() && !endAt.IsZero() && !endAt.After(startAt) {
		result.AddFieldFailure("endAt", "End must be after start")
	}
	if weight < 0 {
		result.AddFieldFailure("weight", "Weight must be >= 0")
	}
	if locale == "" {
		locale = "all"
	}
	if !allowedLocales[locale] {
		result.AddFieldFailure("locale", "Locale must be all, en, or ru")
	}
	return result
}
