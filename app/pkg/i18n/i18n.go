package i18n

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/Spicy-Bush/fider-tarkov-community/app"
	"github.com/Spicy-Bush/fider-tarkov-community/app/assets"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"github.com/gotnospirit/messageformat"
)

var localeToPlurals = map[string]string{
	"en":    "en",
	"pt-BR": "pt",
	"sv-SE": "se",
	"es-ES": "es",
	"el":    "el",
	"nl":    "nl",
	"de":    "de",
	"fr":    "fr",
	"pl":    "pl",
	"ru":    "ru",
	"sk":    "sk",
	"it":    "it",
	"tr":    "tr",
	"zh-CN": "zh",
}

type Params map[string]any

type localeData struct {
	file   map[string]string
	parser *messageformat.Parser
}

func getLocaleData(locale string) localeData {
	content, err := fs.ReadFile(assets.FS, fmt.Sprintf("locale/%s/server.json", locale))
	if err != nil {
		panic(errors.Wrap(err, "failed to read locale file"))
	}

	var file map[string]string
	err = json.Unmarshal(content, &file)
	if err != nil {
		panic(errors.Wrap(err, "failed unmarshal to json"))
	}

	parser, err := messageformat.NewWithCulture(localeToPlurals[locale])
	if err != nil {
		panic(errors.Wrap(err, "failed create parser"))
	}

	return localeData{file, parser}
}

func getMessage(locale, key string) (string, *messageformat.Parser) {
	localeData := getLocaleData(locale)
	if str, ok := localeData.file[key]; ok && str != "" {
		return str, localeData.parser
	}

	enData := getLocaleData("en")
	if str, ok := enData.file[key]; ok && str != "" {
		return str, enData.parser
	}

	return fmt.Sprintf("Missing Translation: %s", key), enData.parser
}

func IsValidLocale(locale string) bool {
	if _, ok := localeToPlurals[locale]; ok {
		return true
	}
	return false
}

func GetLocale(ctx context.Context) string {
	locale, ok := ctx.Value(app.LocaleCtxKey).(string)
	if ok && locale != "" {
		return locale
	}
	return env.Config.Locale
}

func T(ctx context.Context, key string, params ...Params) string {
	locale := GetLocale(ctx)
	msg, parser := getMessage(locale, key)
	if len(params) == 0 {
		return msg
	}

	parsedMsg, err := parser.Parse(msg)
	if err != nil {
		panic(errors.Wrap(err, "failed to parse msg %s", msg))
	}

	str, err := parsedMsg.FormatMap(params[0])
	if err != nil {
		panic(errors.Wrap(err, "failed to format msg '%s' with params '%v'", msg, params[0]))
	}

	return str
}
