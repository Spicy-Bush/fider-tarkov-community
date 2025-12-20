package tpl

import (
	"errors"
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/crypto"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/i18n"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/markdown"
	"github.com/microcosm-cc/bluemonday"
)

var strictHtmlPolicy = bluemonday.NewPolicy()

var cssURLRegex = regexp.MustCompile(`url\s*\(\s*['"]?([^'")\s]+)['"]?\s*\)`)

var cssImageCache sync.Map
var cssFontCache sync.Map

// matches on @font-face blocks and extracts woff2 URLs for preloading
var fontFaceRegex = regexp.MustCompile(`@font-face\s*\{[^}]*src:[^}]*url\s*\(\s*['"]?([^'")\s]+\.woff2[^'")\s]*)['"]?\s*\)[^}]*\}`)

func extractCSSFonts(css string) []string {
	if css == "" {
		return nil
	}

	cacheKey := crypto.MD5(css)

	if cached, ok := cssFontCache.Load(cacheKey); ok {
		return cached.([]string)
	}

	matches := fontFaceRegex.FindAllStringSubmatch(css, -1)
	if len(matches) == 0 {
		cssFontCache.Store(cacheKey, []string{})
		return nil
	}

	urls := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			url := match[1]
			if _, exists := seen[url]; !exists {
				seen[url] = struct{}{}
				urls = append(urls, url)
			}
		}
	}

	cssFontCache.Store(cacheKey, urls)
	return urls
}

func extractCSSImages(css string) []string {
	if css == "" {
		return nil
	}

	cacheKey := crypto.MD5(css)

	if cached, ok := cssImageCache.Load(cacheKey); ok {
		return cached.([]string)
	}

	matches := cssURLRegex.FindAllStringSubmatch(css, -1)
	if len(matches) == 0 {
		return nil
	}

	urls := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			url := match[1]
			if _, exists := seen[url]; !exists && isImageURL(url) {
				seen[url] = struct{}{}
				urls = append(urls, url)
			}
		}
	}

	cssImageCache.Store(cacheKey, urls)
	return urls
}

var templateFunctions = map[string]any{
	"stripHtml": func(input string) string {
		return strictHtmlPolicy.Sanitize(input)
	},
	"html": func(input string) template.HTML {
		return template.HTML(input)
	},
	"safeCSS": func(input string) template.CSS {
		return template.CSS(minifyCSS(input))
	},
	"extractCSSImages": extractCSSImages,
	"extractCSSFonts":  extractCSSFonts,
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

var imageExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".avif", ".ico"}
var imagePaths = []string{"/images/", "/image/", "/attachments/", "/misc/"}

func isImageURL(url string) bool {
	if strings.HasPrefix(url, "data:") {
		return false
	}

	lowerURL := strings.ToLower(url)

	cleanURL := lowerURL
	if idx := strings.IndexByte(lowerURL, '?'); idx != -1 {
		cleanURL = lowerURL[:idx]
	}

	for _, ext := range imageExtensions {
		if strings.HasSuffix(cleanURL, ext) {
			return true
		}
	}

	for _, path := range imagePaths {
		if strings.Contains(lowerURL, path) {
			return true
		}
	}
	return false
}

func minifyCSS(css string) string {
	var b strings.Builder
	b.Grow(len(css))

	inComment := false
	lastWasSpace := false

	for i := 0; i < len(css); i++ {
		c := css[i]

		if inComment {
			if c == '*' && i+1 < len(css) && css[i+1] == '/' {
				inComment = false
				i++
			}
			continue
		}

		if c == '/' && i+1 < len(css) && css[i+1] == '*' {
			inComment = true
			i++
			continue
		}

		isSpace := c == ' ' || c == '\t' || c == '\n' || c == '\r'

		if isSpace {
			if !lastWasSpace && b.Len() > 0 {
				lastWasSpace = true
			}
			continue
		}

		isSpecial := c == '{' || c == '}' || c == ';' || c == ':' || c == ',' || c == '>' || c == '~' || c == '+'

		if isSpecial {
			lastWasSpace = false
		} else if lastWasSpace {
			b.WriteByte(' ')
			lastWasSpace = false
		}

		b.WriteByte(c)
	}

	return b.String()
}
