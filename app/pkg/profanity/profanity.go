package profanity

import (
	"context"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

func ContainsProfanity(ctx context.Context, content string) ([]string, error) {
	getTenantProfanity := &query.GetTenantProfanityWords{}
	if err := bus.Dispatch(ctx, getTenantProfanity); err != nil {
		return nil, err
	}

	profanityList := getTenantProfanity.Result
	if profanityList == "" {
		return nil, nil
	}

	words := strings.Split(profanityList, ",")

	var matches []string
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}

		if strings.Contains(strings.ToLower(content), strings.ToLower(word)) {
			matches = append(matches, word)
		}
	}

	return matches, nil
}
