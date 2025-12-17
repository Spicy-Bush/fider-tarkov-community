package pages

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

var slugRegex = regexp.MustCompile(`[^a-z0-9-]+`)

func GenerateSlug(ctx context.Context, title string, customSlug string, existingPageID int) (string, error) {
	var slug string
	
	if customSlug != "" {
		slug = sanitizeSlug(customSlug)
	} else {
		slug = sanitizeSlug(title)
	}

	isUnique, err := isSlugUnique(ctx, slug, existingPageID)
	if err != nil {
		return "", err
	}

	if isUnique {
		return slug, nil
	}

	counter := 1
	for {
		newSlug := fmt.Sprintf("%s-%d", slug, counter)
		isUnique, err := isSlugUnique(ctx, newSlug, existingPageID)
		if err != nil {
			return "", err
		}
		if isUnique {
			return newSlug, nil
		}
		counter++
	}
}

func sanitizeSlug(input string) string {
	slug := strings.ToLower(input)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = slugRegex.ReplaceAllString(slug, "")
	slug = strings.Trim(slug, "-")
	
	if len(slug) > 200 {
		slug = slug[:200]
		slug = strings.TrimRight(slug, "-")
	}
	
	return slug
}

func isSlugUnique(ctx context.Context, slug string, existingPageID int) (bool, error) {
	getPage := &query.GetPageBySlug{Slug: slug}
	err := bus.Dispatch(ctx, getPage)
	
	if err != nil {
		return true, nil
	}
	
	if getPage.Result != nil && getPage.Result.ID == existingPageID {
		return true, nil
	}
	
	return false, nil
}

