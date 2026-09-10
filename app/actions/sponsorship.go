package actions

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

var allowedSlots = map[string]bool{
	"feed_native": true, "sidebar_top": true, "post_below_title": true, "pages_header": true,
}

var allowedLocales = map[string]bool{"all": true, "en": true, "ru": true}

func isCollaboratorPlus(user *entity.User) bool {
	return user != nil && (user.IsAdministrator() || user.IsCollaborator())
}

// NormalizeSlotsCSV validates and canonicalizes a comma-separated slot list.
func NormalizeSlotsCSV(raw string) (string, bool) {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		if !allowedSlots[s] || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	if len(out) == 0 {
		return "", false
	}
	return strings.Join(out, ","), true
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

type CreateSponsorshipCampaign struct {
	Name             string   `json:"name"`
	Slots            string   `json:"slots"`
	SlotID           string   `json:"slotId"` // legacy single-slot clients
	SlotList         []string `json:"slotList"`
	CreativeImageURL string   `json:"creativeImageUrl"`
	CreativeHTML     string   `json:"creativeHtml"`
	ClickURL         string   `json:"clickUrl"`
	StartAt          time.Time `json:"startAt"`
	EndAt            time.Time `json:"endAt"`
	Weight           int      `json:"weight"`
	Locale           string   `json:"locale"`
	Enabled          bool     `json:"enabled"`
	PackageID        *int     `json:"packageId"`
}

func (a *CreateSponsorshipCampaign) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return isCollaboratorPlus(user)
}

func (a *CreateSponsorshipCampaign) Validate(ctx context.Context, user *entity.User) *validate.Result {
	a.Slots = coalesceSlots(a.Slots, a.SlotID, a.SlotList)
	result := validateCampaignFields(validate.Success(), a.Name, a.Slots, a.CreativeImageURL, a.CreativeHTML, a.ClickURL, a.StartAt, a.EndAt, a.Weight, a.Locale)
	if normalized, ok := NormalizeSlotsCSV(a.Slots); ok {
		a.Slots = normalized
	}
	return result
}

type UpdateSponsorshipCampaign struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	Slots            string   `json:"slots"`
	SlotID           string   `json:"slotId"`
	SlotList         []string `json:"slotList"`
	CreativeImageURL string   `json:"creativeImageUrl"`
	CreativeHTML     string   `json:"creativeHtml"`
	ClickURL         string   `json:"clickUrl"`
	StartAt          time.Time `json:"startAt"`
	EndAt            time.Time `json:"endAt"`
	Weight           int      `json:"weight"`
	Locale           string   `json:"locale"`
	Enabled          bool     `json:"enabled"`
	PackageID        *int     `json:"packageId"`
}

func (a *UpdateSponsorshipCampaign) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return isCollaboratorPlus(user)
}

func (a *UpdateSponsorshipCampaign) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()
	if a.ID <= 0 {
		result.AddFieldFailure("id", "Invalid ID")
	}
	a.Slots = coalesceSlots(a.Slots, a.SlotID, a.SlotList)
	result = validateCampaignFields(result, a.Name, a.Slots, a.CreativeImageURL, a.CreativeHTML, a.ClickURL, a.StartAt, a.EndAt, a.Weight, a.Locale)
	if normalized, ok := NormalizeSlotsCSV(a.Slots); ok {
		a.Slots = normalized
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

func coalesceSlots(slotsCSV, slotID string, slotList []string) string {
	if len(slotList) > 0 {
		return strings.Join(slotList, ",")
	}
	if strings.TrimSpace(slotsCSV) != "" {
		return slotsCSV
	}
	return slotID
}

func validateCampaignFields(result *validate.Result, name, slotsCSV, imageURL, html, clickURL string, startAt, endAt time.Time, weight int, locale string) *validate.Result {
	if strings.TrimSpace(name) == "" {
		result.AddFieldFailure("name", "Name is required")
	}
	normalized, ok := NormalizeSlotsCSV(slotsCSV)
	if !ok {
		result.AddFieldFailure("slots", "Select at least one valid slot")
	} else {
		// write-back via pointer not available; callers use Normalize after validate
		_ = normalized
	}
	if strings.TrimSpace(imageURL) == "" && strings.TrimSpace(html) == "" {
		result.AddFieldFailure("creativeImageUrl", "Provide an image URL or HTML creative")
	}
	if strings.TrimSpace(clickURL) == "" {
		result.AddFieldFailure("clickUrl", "Click URL is required")
	} else if _, err := url.ParseRequestURI(clickURL); err != nil {
		result.AddFieldFailure("clickUrl", "Click URL must be absolute")
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
