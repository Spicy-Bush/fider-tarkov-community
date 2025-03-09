package postgres

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
	"github.com/lib/pq"
)

var onlyalphanumeric = regexp.MustCompile("[^a-zA-Z0-9 |]+")
var replaceOr = strings.NewReplacer("|", " ")

// ToTSQuery converts input to another string that can be safely used for ts_query
func ToTSQuery(input string) string {
	input = replaceOr.Replace(onlyalphanumeric.ReplaceAllString(input, ""))
	return strings.Join(strings.Fields(input), "|")
}

// SanitizeString converts input to another string that only contains utf-8 characters and not-null characters
func SanitizeString(input string) string {
	input = strings.Replace(input, "\u0000", "", -1)
	return strings.ToValidUTF8(input, "")
}

func getViewData(query query.SearchPosts, userID int) (string, []enum.PostStatus, string, []interface{}) {
	var sort string
	statusFilters := query.Statuses
	if len(statusFilters) == 0 {
		// Use a sensible default list of status filters
		statusFilters = []enum.PostStatus{
			enum.PostOpen,
			enum.PostStarted,
			enum.PostPlanned,
		}
	}

	extraParams := []interface{}{}
	paramIndex := 3
	conditions := []string{}

	if query.MyVotesOnly {
		conditions = append(conditions, "vote_type IS NOT NULL")
	}
	if query.MyPostsOnly {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", paramIndex))
		extraParams = append(extraParams, userID)
		paramIndex++
	}
	if len(query.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("tags && $%d", paramIndex))
		// Note: We pass the tags array here
		extraParams = append(extraParams, pq.Array(query.Tags))
		paramIndex++
	}

	if query.Date != "" {
		var interval string
		switch query.Date {
		case "1d":
			interval = "1 day"
		case "7d":
			interval = "7 days"
		case "30d":
			interval = "30 days"
		case "6m":
			interval = "6 months"
		case "1y":
			interval = "1 year"
		}

		if interval != "" {
			conditions = append(conditions, fmt.Sprintf("created_at >= NOW() - INTERVAL '%s'", interval))
		}
	}

	condition := ""
	if len(conditions) > 0 {
		condition = "AND " + strings.Join(conditions, " AND ")
	}

	switch query.View {
	case "recent":
		sort = "id"
	case "most-wanted":
		sort = "votes_count"
	case "most-discussed":
		sort = "comments_count"
	case "my-votes":
		// Depracated: Use status filters instead
		sort = "id"
	case "planned":
		// Depracated: Use status filters instead
		sort = "response_date"
		statusFilters = []enum.PostStatus{enum.PostPlanned}
	case "started":
		// Depracated: Use status filters instead
		sort = "response_date"
		statusFilters = []enum.PostStatus{enum.PostStarted}
	case "completed":
		// Depracated: Use status filters instead
		sort = "response_date"
		statusFilters = []enum.PostStatus{enum.PostCompleted}
	case "declined":
		// Depracated: Use status filters instead
		sort = "response_date"
		statusFilters = []enum.PostStatus{enum.PostDeclined}
	case "all":
		sort = "id"
		statusFilters = []enum.PostStatus{
			enum.PostOpen,
			enum.PostStarted,
			enum.PostPlanned,
			enum.PostCompleted,
			enum.PostDeclined,
		}
	case "controversial":
		sort = "CASE " +
			"WHEN upvotes > 0 AND downvotes > 0 THEN " +
			"(downvotes::float / GREATEST(upvotes, 1)) * " +
			"(upvotes + downvotes) / " +
			"pow((EXTRACT(EPOCH FROM current_timestamp - created_at)/86400) + 1, 0.5) " +
			"ELSE (downvotes::float) / " +
			"pow((EXTRACT(EPOCH FROM current_timestamp - created_at)/86400) + 2, 1.2) " +
			"END"
	case "trending":
		fallthrough
	default:
		sort = "((COALESCE(recent_votes_count, 0)*5 + COALESCE(recent_comments_count, 0) *3)-1) / " +
			"pow((EXTRACT(EPOCH FROM current_timestamp - created_at)/3600) + 2, 1.4)"
	}
	return condition, statusFilters, sort, extraParams
}

func buildAvatarURL(ctx context.Context, avatarType enum.AvatarType, id int, name, avatarBlobKey string) string {
	if name == "" {
		name = "-"
	}

	if avatarType == enum.AvatarTypeCustom {
		return web.AssetsURL(ctx, "/static/images/%s", avatarBlobKey)
	} else {
		return web.AssetsURL(ctx, "/static/avatars/%s/%d/%s", avatarType.String(), id, url.PathEscape(name))
	}
}
