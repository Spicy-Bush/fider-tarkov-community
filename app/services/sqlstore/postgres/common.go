package postgres

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/web"
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
