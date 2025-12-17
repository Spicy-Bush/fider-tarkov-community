package actions

import (
	"context"
	"net/url"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/entity"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/validate"
)

type NavigationLinkInput struct {
	Title        string `json:"title"`
	URL          string `json:"url"`
	DisplayOrder int    `json:"displayOrder"`
	Location     string `json:"location"`
}

type SaveNavigationLinks struct {
	Links []NavigationLinkInput `json:"links"`
}

func (action *SaveNavigationLinks) IsAuthorized(ctx context.Context, user *entity.User) bool {
	return user != nil && user.IsAdministrator()
}

func (action *SaveNavigationLinks) Validate(ctx context.Context, user *entity.User) *validate.Result {
	result := validate.Success()

	validLocations := map[string]bool{"footer": true, "subheader": true}

	for i, link := range action.Links {
		if len(strings.TrimSpace(link.Title)) == 0 {
			result.AddFieldFailure("links", "Link title is required")
			continue
		}
		if len(link.Title) > 100 {
			result.AddFieldFailure("links", "Link title must be less than 100 characters")
			continue
		}

		if len(strings.TrimSpace(link.URL)) == 0 {
			result.AddFieldFailure("links", "Link URL is required")
			continue
		}
		if len(link.URL) > 500 {
			result.AddFieldFailure("links", "Link URL must be less than 500 characters")
			continue
		}

		if !isValidNavigationURL(link.URL) {
			result.AddFieldFailure("links", "Link URL must be a valid relative path or HTTP(S) URL")
			continue
		}

		if !validLocations[link.Location] {
			result.AddFieldFailure("links", "Link location must be 'footer' or 'subheader'")
			continue
		}

		if i > 50 {
			result.AddFieldFailure("links", "Maximum 50 navigation links allowed")
			break
		}
	}

	return result
}

func isValidNavigationURL(rawURL string) bool {
	if strings.HasPrefix(rawURL, "/") {
		return !strings.Contains(rawURL, "..") && !strings.Contains(rawURL, "//")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "mailto" {
		return false
	}

	if scheme == "mailto" {
		return strings.Contains(rawURL, "@")
	}

	return parsed.Host != ""
}

