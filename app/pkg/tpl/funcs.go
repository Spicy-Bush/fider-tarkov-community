package tpl

import (
	"errors"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/crypto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/microcosm-cc/bluemonday"
)

var strictHtmlPolicy = bluemonday.NewPolicy()

var templateFunctions = map[string]any{
	"stripHtml": func(input string) string {
		return strictHtmlPolicy.Sanitize(input)
	},
	"html": func(input string) template.HTML {
		return template.HTML(input)
	},
	"md5": func(input string) string {
		return crypto.MD5(input)
	},
	"lower": func(input string) string {
		return strings.ToLower(input)
	},
	"upper": func(input string) string {
		return strings.ToUpper(input)
	},
	"translate": func(input string, params ...i18n.Params) string {
		return "This is overwritten later on..."
	},
	"markdown": func(input string) template.HTML {
		return markdown.Full(input)
	},
	"dict": func(values ...any) map[string]any {
		if len(values)%2 != 0 {
			panic(errors.New("invalid dictionary call"))
		}

		dict := make(map[string]any)
		for i := 0; i < len(values); i += 2 {
			var key string
			switch v := values[i].(type) {
			case string:
				key = v
			default:
				panic(errors.New("invalid dictionary key"))
			}
			dict[key] = values[i+1]
		}
		return dict
	},
	"format": func(format string, date time.Time) string {
		return date.Format(format)
	},
	"quote": func(text any) string {
		return quote(text)
	},
	"escape": func(text any) string {
		quoted := quote(text)
		return quoted[1 : len(quoted)-1]
	},
	"truncate": func(input string, length int) string {
		if len([]rune(input)) > length {
			input = string([]rune(input)[:length-3]) + "..."
		}
		return input
	},
}

func quote(text any) string {
	return strconv.Quote(fmt.Sprintf("%v", text))
}
