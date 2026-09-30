package adsselect

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
)

func Reserves(c *entity.SponsorCampaign, now time.Time) bool {
	if !c.EndAt.After(now) {
		return false
	}

	switch c.State {
	case "booked", "paused":
		return true
	case "reserved":
		return c.ConfirmBy != nil && c.ConfirmBy.After(now)
	default:
		return false
	}
}

func Matches(c *entity.SponsorCampaign, pageType, language, country string) bool {
	return slices.Contains(c.PageTypes, pageType) &&
		(len(c.Languages) == 0 || slices.Contains(c.Languages, language)) &&
		(len(c.Countries) == 0 || slices.Contains(c.Countries, country))
}

func CheckAvailability(candidate *entity.SponsorCampaign, campaigns []*entity.SponsorCampaign, now time.Time) error {
	if !Reserves(candidate, now) {
		return nil
	}

	countries := append([]string{""}, candidate.Countries...)
	for _, c := range campaigns {
		countries = append(countries, c.Countries...)
	}
	slices.Sort(countries)
	countries = slices.Compact(countries)

	for _, booking := range candidate.Bookings {
		for _, pageType := range candidate.PageTypes {
			for _, language := range []string{"en", "ru"} {
				for _, country := range countries {
					if !Matches(candidate, pageType, language, country) {
						continue
					}

					type change struct {
						at    time.Time
						share int
					}
					changes := []change{{candidate.StartAt, booking.Share}}
					for _, other := range campaigns {
						if other.ID == candidate.ID || !Reserves(other, now) || !other.EndAt.After(candidate.StartAt) || !candidate.EndAt.After(other.StartAt) || !Matches(other, pageType, language, country) {
							continue
						}

						for _, existing := range other.Bookings {
							if existing.PlacementID != booking.PlacementID {
								continue
							}

							if (candidate.Exclusive || other.Exclusive) && candidate.Category == other.Category && !strings.EqualFold(candidate.Advertiser, other.Advertiser) {
								return fmt.Errorf("%s overlaps the category-exclusive booking %q.", booking.PlacementID, other.Name)
							}

							start := other.StartAt
							if start.Before(candidate.StartAt) {
								start = candidate.StartAt
							}
							changes = append(changes, change{start, existing.Share}, change{other.EndAt, -existing.Share})
						}
					}

					slices.SortFunc(changes, func(a, b change) int { return a.at.Compare(b.at) })
					total := 0
					for i := 0; i < len(changes); {
						at := changes[i].at
						for i < len(changes) && changes[i].at.Equal(at) {
							total += changes[i].share
							i++
						}

						if at.Before(candidate.EndAt) && total > 100 {
							return fmt.Errorf("%s would be booked at %d percent for an overlapping audience.", booking.PlacementID, total)
						}
					}
				}
			}
		}
	}

	return nil
}
