package actions

import (
	"context"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/blob"
)

type UploadSponsorImage UploadNewFile

func (a *UploadSponsorImage) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return sponsorAuthorised(ctx, user)
}

func (a *UploadSponsorImage) Validate(ctx context.Context, user *entity.User) *validate.Result {
	a.UploadType = string(enum.FileUploadPublic)
	return (*UploadNewFile)(a).Validate(ctx, user)
}

type SaveSponsorCampaign struct {
	BaseCampaign *entity.SponsorCampaign `json:"baseCampaign"`
	BaseCreative *entity.SponsorCreative `json:"baseCreative"`
	Campaign     entity.SponsorCampaign  `json:"campaign"`
	Creative     *entity.SponsorCreative `json:"creative"`
	SubmissionID string                  `json:"submissionId"`
}

func sponsorAuthorised(ctx context.Context, user *entity.User) bool {
	tenant, _ := ctx.Value(app.TenantCtxKey).(*entity.Tenant)
	return entity.Can(user, tenant, entity.ManageSponsorship)
}

func (a *SaveSponsorCampaign) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return sponsorAuthorised(ctx, user)
}

func (a *SaveSponsorCampaign) Validate(ctx context.Context, user *entity.User) *validate.Result {
	r := validate.Success()
	c := &a.Campaign
	c.Name = strings.TrimSpace(c.Name)
	c.Advertiser = strings.TrimSpace(c.Advertiser)
	c.Category = strings.ToLower(strings.TrimSpace(c.Category))
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.StartAt = c.StartAt.UTC()
	c.EndAt = c.EndAt.UTC()

	if !validate.ValidSubmissionID(a.SubmissionID) {
		r.AddFieldFailure("submissionId", "A save identity is required.")
	}

	if c.ID < 0 || (c.ID > 0 && c.Revision < 1) {
		r.AddFieldFailure("campaign", "Reload this booking before saving.")
	}

	if c.Name == "" || len(c.Name) > 160 || c.Advertiser == "" || len(c.Advertiser) > 160 {
		r.AddFieldFailure("name", "Enter a booking name and advertiser, each under 161 characters.")
	}

	if len(c.Category) > 80 || (c.Exclusive && c.Category == "") {
		r.AddFieldFailure("category", "Enter a category for exclusive bookings.")
	}

	if !slices.Contains([]string{"draft", "reserved", "booked", "paused", "cancelled"}, c.State) {
		r.AddFieldFailure("state", "Choose a booking status.")
	}

	if c.StartAt.IsZero() || !c.EndAt.After(c.StartAt) {
		r.AddFieldFailure("endAt", "The end must be after the start.")
	}

	if c.State == "reserved" && (c.ConfirmBy == nil || c.ConfirmBy.After(c.StartAt)) {
		r.AddFieldFailure("confirmBy", "Set a confirmation deadline no later than the reserved start.")
	}

	if len(c.Bookings) == 0 || len(c.Bookings) > len(entity.SponsorPlacements) {
		r.AddFieldFailure("bookings", "Choose the placements to book.")
	}

	seen := make(map[string]bool)
	for _, booking := range c.Bookings {
		valid := slices.ContainsFunc(entity.SponsorPlacements, func(p entity.SponsorPlacement) bool {
			return p.ID == booking.PlacementID
		})
		if !valid || seen[booking.PlacementID] || booking.Share < 1 || booking.Share > 100 {
			r.AddFieldFailure("bookings", "Use each placement once with a share from 1 to 100 percent.")
			break
		}

		seen[booking.PlacementID] = true
	}

	for _, language := range c.Languages {
		if !slices.Contains([]string{"en", "ru"}, language) {
			r.AddFieldFailure("languages", "Choose English, Russian or all languages.")
			break
		}
	}

	if len(c.Countries) > 250 || len(c.Languages) > 2 || len(c.PageTypes) == 0 || len(c.PageTypes) > 3 {
		r.AddFieldFailure("targeting", "Choose valid audience filters.")
	}

	for _, country := range c.Countries {
		if len(country) != 2 || country[0] < 'A' || country[0] > 'Z' || country[1] < 'A' || country[1] > 'Z' {
			r.AddFieldFailure("countries", "Use two-letter country codes.")
			break
		}
	}

	for _, pageType := range c.PageTypes {
		if !slices.Contains([]string{"home", "post", "page"}, pageType) {
			r.AddFieldFailure("pageTypes", "Choose home, posts or Pages.")
			break
		}
	}

	if c.AmountMinor < 0 || c.AmountMinor > 1_000_000_000_000 || len(c.Currency) != 3 || strings.IndexFunc(c.Currency, func(r rune) bool { return r < 'A' || r > 'Z' }) >= 0 {
		r.AddFieldFailure("amountMinor", "Enter a valid amount and three-letter currency code.")
	}

	if !slices.Contains([]string{"unpaid", "partial", "paid"}, c.PaymentStatus) {
		r.AddFieldFailure("paymentStatus", "Choose a payment status.")
	}

	if len(c.Notes) > 4000 {
		r.AddFieldFailure("notes", "Keep booking notes under 4001 characters.")
	}

	if a.Creative != nil {
		if a.Creative.CampaignID != c.ID || (c.ID == 0 && (a.Creative.ID != 0 || a.Creative.Revision != 0)) {
			r.AddFieldFailure("creative", "Choose artwork belonging to this campaign.")
		}

		validateSponsorCreative(a.Creative, r)
	}

	return r
}

func validateSponsorCreative(c *entity.SponsorCreative, r *validate.Result) {
	c.Headline = strings.TrimSpace(c.Headline)
	c.Destination = strings.TrimSpace(c.Destination)

	if !slices.Contains([]string{"draft", "review", "approved", "rejected"}, c.State) {
		r.AddFieldFailure("state", "Choose a review status.")
	}

	if c.State == "rejected" && strings.TrimSpace(c.ReviewReason) == "" {
		r.AddFieldFailure("reviewReason", "Explain why this creative was rejected.")
	}

	if !slices.Contains([]string{"", "en", "ru"}, c.Language) || !slices.Contains([]string{"", "desktop", "mobile"}, c.Device) {
		r.AddFieldFailure("language", "Choose a supported language and device.")
	}

	if c.ImageKey == "" && c.Headline == "" {
		r.AddFieldFailure("imageKey", "Choose an image or enter a headline.")
	}

	if crop := c.BannerCrop; crop != nil {
		if crop.X < 0 || crop.X > 100 || crop.Y < 0 || crop.Y > 100 || crop.Zoom < 1 || crop.Zoom > 3 {
			r.AddFieldFailure("bannerCrop", "Choose a crop within the image and a zoom between 1 and 3.")
		}
	}

	if len(c.Headline) > 120 || len(c.Description) > 400 || len(c.CallToAction) > 40 || len(c.ReviewReason) > 1000 {
		r.AddFieldFailure("headline", "Use a headline up to 120 characters, description up to 400 and button label up to 40.")
	}

	destination, err := url.Parse(c.Destination)
	if err != nil || destination.Hostname() == "" || destination.User != nil || (destination.Scheme != "https" && destination.Scheme != "http") || len(c.Destination) > 2048 || strings.IndexFunc(c.Destination, unicode.IsControl) >= 0 {
		r.AddFieldFailure("destination", "Enter an HTTP or HTTPS destination")
	}

	for _, key := range []string{c.ImageKey, c.LogoKey} {
		if key != "" && blob.ValidateKey(key) != nil {
			r.AddFieldFailure("imageKey", "Choose an image from this site's library.")
			break
		}
	}

	if c.StartAt != nil && c.EndAt != nil && !c.EndAt.After(*c.StartAt) {
		r.AddFieldFailure("endAt", "The creative end must be after its start.")
	}

	if len(c.OfferCode) > 80 || len(c.OfferTerms) > 500 || (c.OfferCode != "" && (c.OfferExpires == nil || strings.TrimSpace(c.OfferTerms) == "")) {
		r.AddFieldFailure("offerCode", "Offers need eligibility terms and an expiry date.")
	}

}

type SaveSponsorPlacement struct {
	SubmissionID string                  `json:"submissionId"`
	Placement    entity.SponsorPlacement `json:"placement"`
}

var adSenseSlotID = regexp.MustCompile(`^[0-9]{1,20}$`)

func (a *SaveSponsorPlacement) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return sponsorAuthorised(ctx, user)
}

func (a *SaveSponsorPlacement) Validate(ctx context.Context, user *entity.User) *validate.Result {
	r := validate.Success()
	if !validate.ValidSubmissionID(a.SubmissionID) {
		r.AddFieldFailure("submissionId", "A save identity is required.")
	}

	p := a.Placement
	i := slices.IndexFunc(entity.SponsorPlacements, func(item entity.SponsorPlacement) bool { return item.ID == p.ID })
	if i < 0 {
		r.AddFieldFailure("placement", "Choose an existing placement.")
		return r
	}

	catalog := entity.SponsorPlacements[i]
	validPosition := p.Position == catalog.Position
	if catalog.PageType == "post" || catalog.PageType == "page" {
		validPosition = slices.Contains([]string{"before", "within", "after"}, p.Position)
	}

	if !validPosition || !slices.Contains([]string{"none", "kofi", "adsense"}, p.Empty) {
		r.AddFieldFailure("placement", "Choose a supported position and empty-slot option.")
	}

	if p.Every < 0 || p.Every > 100 || (p.Position == "feed" && p.Every < 8) || (p.Position == "within" && p.Every < 1) {
		r.AddFieldFailure("every", "Feed cards need at least eight posts between them; discussion panels need at least one thread.")
	}

	a.Placement = catalog
	p.AdSenseSlotID = strings.TrimSpace(p.AdSenseSlotID)
	if (p.Empty == "adsense" || p.AdSenseSlotID != "") && !adSenseSlotID.MatchString(p.AdSenseSlotID) {
		r.AddFieldFailure("adsenseSlotId", "Enter the numeric AdSense ad unit ID.")
	}
	a.Placement.AdSenseSlotID = p.AdSenseSlotID
	a.Placement.Enabled = p.Enabled
	a.Placement.Position = p.Position
	a.Placement.Every = p.Every
	a.Placement.Empty = p.Empty
	return r
}
