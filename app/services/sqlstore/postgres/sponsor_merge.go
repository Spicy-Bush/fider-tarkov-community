package postgres

import (
	"slices"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

func mergeSponsorField[T comparable](name string, base, saved T, edited *T, conflicts *[]string) {
	if *edited == base {
		*edited = saved
	} else if saved != base && saved != *edited {
		*conflicts = append(*conflicts, name)
	}
}

func mergeSponsorChoices(name string, base, saved []string, edited *[]string, conflicts *[]string) {
	if slices.Equal(*edited, base) {
		*edited = saved
	} else if !slices.Equal(saved, base) && !slices.Equal(saved, *edited) {
		*conflicts = append(*conflicts, name)
	}
}

func sameSponsorDate(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == right
	}

	return left.Equal(*right)
}

func mergeSponsorDate(name string, base, saved *time.Time, edited **time.Time, conflicts *[]string) {
	if sameSponsorDate(*edited, base) {
		*edited = saved
	} else if !sameSponsorDate(saved, base) && !sameSponsorDate(saved, *edited) {
		*conflicts = append(*conflicts, name)
	}
}

func mergeSponsorCampaign(base, edited, saved entity.SponsorCampaign) (entity.SponsorCampaign, entity.SponsorConflicts) {
	var conflicts entity.SponsorConflicts
	fields := &conflicts.Campaign
	mergeSponsorField("name", base.Name, saved.Name, &edited.Name, fields)
	mergeSponsorField("advertiser", base.Advertiser, saved.Advertiser, &edited.Advertiser, fields)
	mergeSponsorField("category", base.Category, saved.Category, &edited.Category, fields)
	mergeSponsorField("exclusive", base.Exclusive, saved.Exclusive, &edited.Exclusive, fields)
	mergeSponsorField("state", base.State, saved.State, &edited.State, fields)
	mergeSponsorField("amountMinor", base.AmountMinor, saved.AmountMinor, &edited.AmountMinor, fields)
	mergeSponsorField("currency", base.Currency, saved.Currency, &edited.Currency, fields)
	mergeSponsorField("paymentStatus", base.PaymentStatus, saved.PaymentStatus, &edited.PaymentStatus, fields)
	mergeSponsorField("notes", base.Notes, saved.Notes, &edited.Notes, fields)

	start, end := &edited.StartAt, &edited.EndAt
	mergeSponsorDate("startAt", &base.StartAt, &saved.StartAt, &start, fields)
	mergeSponsorDate("endAt", &base.EndAt, &saved.EndAt, &end, fields)
	edited.StartAt, edited.EndAt = *start, *end
	mergeSponsorDate("confirmBy", base.ConfirmBy, saved.ConfirmBy, &edited.ConfirmBy, fields)
	mergeSponsorChoices("languages", base.Languages, saved.Languages, &edited.Languages, fields)
	mergeSponsorChoices("countries", base.Countries, saved.Countries, &edited.Countries, fields)
	mergeSponsorChoices("pageTypes", base.PageTypes, saved.PageTypes, &edited.PageTypes, fields)

	shares := func(bookings []entity.SponsorBooking) map[string]int {
		result := make(map[string]int, len(bookings))
		for _, booking := range bookings {
			result[booking.PlacementID] = booking.Share
		}

		return result
	}
	original, current, desired := shares(base.Bookings), shares(saved.Bookings), shares(edited.Bookings)
	edited.Bookings = nil
	for _, placement := range entity.SponsorPlacements {
		share := desired[placement.ID]
		mergeSponsorField(placement.ID, original[placement.ID], current[placement.ID], &share, &conflicts.Bookings)
		if share > 0 {
			edited.Bookings = append(edited.Bookings, entity.SponsorBooking{PlacementID: placement.ID, Share: share})
		}
	}

	edited.ID, edited.Revision = saved.ID, saved.Revision
	return edited, conflicts
}

func mergeSponsorCreative(base, edited, saved entity.SponsorCreative) (entity.SponsorCreative, entity.SponsorConflicts) {
	var conflicts entity.SponsorConflicts
	fields := &conflicts.Creative
	mergeSponsorField("state", base.State, saved.State, &edited.State, fields)
	mergeSponsorField("reviewReason", base.ReviewReason, saved.ReviewReason, &edited.ReviewReason, fields)
	mergeSponsorField("language", base.Language, saved.Language, &edited.Language, fields)
	mergeSponsorField("device", base.Device, saved.Device, &edited.Device, fields)
	mergeSponsorField("framed", base.Framed, saved.Framed, &edited.Framed, fields)
	mergeSponsorField("headline", base.Headline, saved.Headline, &edited.Headline, fields)
	mergeSponsorField("description", base.Description, saved.Description, &edited.Description, fields)
	mergeSponsorField("logoKey", base.LogoKey, saved.LogoKey, &edited.LogoKey, fields)
	mergeSponsorField("callToAction", base.CallToAction, saved.CallToAction, &edited.CallToAction, fields)
	mergeSponsorField("destination", base.Destination, saved.Destination, &edited.Destination, fields)
	mergeSponsorField("offerCode", base.OfferCode, saved.OfferCode, &edited.OfferCode, fields)
	mergeSponsorField("offerTerms", base.OfferTerms, saved.OfferTerms, &edited.OfferTerms, fields)
	mergeSponsorDate("startAt", base.StartAt, saved.StartAt, &edited.StartAt, fields)
	mergeSponsorDate("endAt", base.EndAt, saved.EndAt, &edited.EndAt, fields)
	mergeSponsorDate("offerExpires", base.OfferExpires, saved.OfferExpires, &edited.OfferExpires, fields)

	type image struct {
		key     string
		crop    entity.SponsorImageCrop
		cropped bool
	}
	artwork := func(creative entity.SponsorCreative) image {
		value := image{key: creative.ImageKey, cropped: creative.BannerCrop != nil}
		if value.cropped {
			value.crop = *creative.BannerCrop
		}

		return value
	}
	original, desired, current := artwork(base), artwork(edited), artwork(saved)
	if desired == original {
		edited.ImageKey, edited.BannerCrop = saved.ImageKey, saved.BannerCrop
	} else if current != original && current != desired {
		conflicts.Image = true
	}

	edited.ID, edited.CampaignID, edited.Revision = saved.ID, saved.CampaignID, saved.Revision
	return edited, conflicts
}
