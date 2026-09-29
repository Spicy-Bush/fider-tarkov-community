package postgres

import (
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Spicy-Bush/fider-tarkov-community/app/services/sqlstore/postgres/mediaowners"
)

var mediaCSSHex = regexp.MustCompile(`\\([0-9A-Fa-f]{1,6})[ \t]?`)
var mediaCSSEscape = regexp.MustCompile(`\\([^0-9A-Fa-f\n\r\f])`)
var mediaPaths = regexp.MustCompile(`(?i)(?:/|%2f)[^\s<>"'?#]+`)
var mediaURLWhitespace = strings.NewReplacer("\t", "", "\n", "", "\r", "")

type extractedMediaReference struct {
	Key   string `json:"key"`
	Field string `json:"field"`
}

// References include encoded URLs and literal examples so cleanup cannot remove an image still present in saved content.
func mediaTextKeys(content string) []string {
	if !strings.ContainsAny(content, mediaowners.ReferenceIntroducers) {
		return nil
	}

	decoded := html.UnescapeString(content)
	if strings.ContainsRune(decoded, '\\') {
		decoded = mediaCSSHex.ReplaceAllStringFunc(decoded, func(value string) string {
			point, err := strconv.ParseInt(strings.TrimSpace(value[1:]), 16, 32)
			if err != nil || point == 0 || point > unicode.MaxRune || point >= 0xD800 && point <= 0xDFFF {
				return value
			}
			return string(rune(point))
		})
		decoded = mediaCSSEscape.ReplaceAllString(decoded, "$1")
	}
	decoded = mediaURLWhitespace.Replace(decoded)
	unchangedParentheses := strings.Count(content, "(") == strings.Count(decoded, "(") &&
		strings.Count(content, ")") == strings.Count(decoded, ")") && !strings.Contains(content, `\)`)

	seen := make(map[string]bool)
	add := func(key string) {
		if len(key) == 0 || len(key) > 512 || strings.ContainsFunc(key, func(character rune) bool {
			return unicode.IsSpace(character) || unicode.IsControl(character)
		}) {
			return
		}
		seen[key] = true
	}

	inputs := []string{content}
	if decoded != content {
		inputs = append(inputs, decoded)
	}

	for _, input := range inputs {
		firstPath := strings.IndexAny(input, "/%")
		if firstPath < 0 {
			continue
		}

		for _, match := range mediaPaths.FindAllStringIndex(input[firstPath:], -1) {
			match[0] += firstPath
			match[1] += firstPath
			token := input[match[0]:match[1]]
			prefix := input[:match[0]]
			opening := strings.LastIndexByte(prefix, '(')
			cssURL := opening >= 3 && strings.EqualFold(prefix[opening-3:opening], "url")
			markupURL := unchangedParentheses && opening >= 0 && !strings.ContainsAny(prefix[opening+1:], "() \t\r\n\"'<>") &&
				(strings.HasSuffix(prefix[:opening], "]") || cssURL)
			quotedURL := match[1] < len(input) && (input[match[1]] == '"' || input[match[1]] == '\'')
			if markupURL {
				depth := 0
				for index, character := range token {
					if character == '(' {
						depth++
					} else if character == ')' {
						if depth == 0 {
							token = token[:index]
							break
						}
						depth--
					}
				}
			}
			path, err := url.PathUnescape(token)
			if err != nil || !utf8.ValidString(path) {
				path = token
			}

			for _, route := range []string{"/static/images/", "/static/favicon/"} {
				position := strings.Index(path, route)
				if position < 0 {
					continue
				}

				key := path[position+len(route):]
				add(key)
				if markupURL || quotedURL {
					continue
				}
				for index, character := range key {
					if strings.ContainsRune(")]};,!", character) {
						add(key[:index])
						if strings.ContainsRune(")]}", character) {
							add(key[:index+1])
						}
					}
				}
			}
		}
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func mediaFieldReferences(kind string, stringField func(string) string) []extractedMediaReference {
	var keys, markup []string
	ownerKind, _, _ := strings.Cut(kind, ":")
	for _, owner := range mediaowners.All {
		if owner.Kind == ownerKind {
			keys, markup = owner.Keys, owner.Markup
			break
		}
	}

	references := make([]extractedMediaReference, 0)
	for _, field := range keys {
		if key := stringField(field); key != "" {
			references = append(references, extractedMediaReference{Key: key, Field: field})
		}
	}
	for _, field := range markup {
		for _, key := range mediaTextKeys(stringField(field)) {
			references = append(references, extractedMediaReference{Key: key, Field: field})
		}
	}
	return references
}

func extractMediaReferences(kind string, source string) ([]extractedMediaReference, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(source), &document); err != nil {
		return nil, err
	}

	stringField := func(name string) string {
		value := document[name]
		if len(value) == 0 || string(value) == "null" {
			return ""
		}
		var text string
		if json.Unmarshal(value, &text) == nil {
			return text
		}
		return string(value)
	}

	references := mediaFieldReferences(kind, stringField)
	seen := make(map[extractedMediaReference]bool, len(references))
	for _, reference := range references {
		seen[reference] = true
	}
	add := func(key, field string) {
		reference := extractedMediaReference{Key: key, Field: field}
		if key != "" && !seen[reference] {
			seen[reference] = true
			references = append(references, reference)
		}
	}

	if strings.HasPrefix(kind, "moderation:") {
		switch stringField("state") {
		case "pending", "running", "failed":
			if kind == "moderation:post" || kind == "moderation:comment" {
				for _, key := range mediaTextKeys(stringField("text_content")) {
					add(key, "text_content")
				}
			}

			var blobs []string
			if value := document["blob_keys"]; len(value) > 0 {
				if err := json.Unmarshal(value, &blobs); err != nil {
					return nil, err
				}
			}
			for _, key := range blobs {
				add(key, "blob_keys")
			}

			var fallback struct {
				Avatar string `json:"avatar_bkey"`
			}
			if value := document["fallback_profile"]; len(value) > 0 {
				if err := json.Unmarshal(value, &fallback); err != nil {
					return nil, err
				}
			}
			add(fallback.Avatar, "fallback_profile")
		}
	}

	slices.SortFunc(references, func(a, b extractedMediaReference) int {
		if comparison := strings.Compare(a.Key, b.Key); comparison != 0 {
			return comparison
		}
		return strings.Compare(a.Field, b.Field)
	})
	return references, nil
}
