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

func getViewData(query query.SearchPosts, userID int) (string, []enum.PostStatus, string, string, []interface{}) {
	var sort string
	sortDir := "DESC"
	statusFilters := query.Statuses
	if len(statusFilters) == 0 {
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

	if query.NotMyVotes && userID > 0 {
		conditions = append(conditions, "vote_type IS NULL")
	}

	if query.MyPostsOnly {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", paramIndex))
		extraParams = append(extraParams, userID)
		paramIndex++
	}

	if len(query.Tags) > 0 {
		if query.TagLogic == "AND" {
			for _, tag := range query.Tags {
				conditions = append(conditions, fmt.Sprintf("$%d = ANY(tags)", paramIndex))
				extraParams = append(extraParams, tag)
				paramIndex++
			}
		} else {
			conditions = append(conditions, fmt.Sprintf("tags && $%d", paramIndex))
			extraParams = append(extraParams, pq.Array(query.Tags))
			paramIndex++
		}
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
	case "newest":
		sort = "id"
	case "oldest":
		sort = "id"
		sortDir = "ASC"
	case "recently-updated":
		sort = "CASE WHEN status = " + fmt.Sprintf("%d", int(enum.PostOpen)) + " THEN -999999999 ELSE extract(epoch from COALESCE(response_date, created_at)) END"
	case "most-wanted":
		sort = "votes_count"
	case "least-wanted":
		sort = "votes_count"
		sortDir = "ASC"
	case "most-discussed":
		sort = "comments_count"
	case "planned":
		sort = "response_date"
		statusFilters = []enum.PostStatus{enum.PostPlanned}
	case "started":
		sort = "response_date"
		statusFilters = []enum.PostStatus{enum.PostStarted}
	case "completed":
		sort = "response_date"
		statusFilters = []enum.PostStatus{enum.PostCompleted}
	case "declined":
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
			"WHEN upvotes > 0 OR downvotes > 0 THEN " +
			"(upvotes + downvotes) * (1 - ABS(upvotes - downvotes)::float / GREATEST(upvotes + downvotes, 1)) / " +
			"pow((EXTRACT(EPOCH FROM current_timestamp - created_at)/86400) + 1, 0.5) " +
			"ELSE 0 " +
			"END"
	case "trending":
		fallthrough
	default:
		sort = "(" +
			"COALESCE(recent_comments_count, 0)*3 + " +
			"CASE " +
			"  WHEN COALESCE(recent_votes_count, 0) >= 0 THEN COALESCE(recent_votes_count, 0)*5 " +
			"  WHEN COALESCE(recent_votes_count, 0) > -10 THEN 0 " +
			"  ELSE COALESCE(recent_votes_count, 0)*5 " +
			"END + " +
			"CASE WHEN (upvotes > 20) THEN upvotes/2 ELSE 0 END" +
			") / " +
			"pow((EXTRACT(EPOCH FROM current_timestamp - last_activity_at)/86400) + 2, 0.8)"
	}
	return condition, statusFilters, sort, sortDir, extraParams
}

func buildAvatarURL(ctx context.Context, avatarType enum.AvatarType, id int, name, avatarBlobKey string) string {
	if name == "" {
		name = "-"
	}

	switch avatarType {
	case enum.AvatarTypeCustom:
		return web.AssetsURL(ctx, "/static/images/%s", avatarBlobKey)
	case enum.AvatarTypeGravatar:
		return web.AssetsURL(ctx, "/static/avatars/gravatar/%d/%s", id, url.PathEscape(name))
	default:
		return web.AssetsURL(ctx, "/static/avatars/letter/%d/%s", id, url.PathEscape(name))
	}
}
