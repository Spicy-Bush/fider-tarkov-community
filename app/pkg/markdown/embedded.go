package markdown

import (
	"regexp"
	"strconv"
	"strings"
)

type EmbeddedContent struct {
	Type    string
	Filters map[string]string
	PostIDs []int
	Limit   int
}

var (
	embeddedTableRegex = regexp.MustCompile(`<table\s+type=(\w+)\s+filters=(?:"([^"]+)"|(\S+))(?:\s+limit=(\d+))?\s*/>`)
	embeddedPostRegex  = regexp.MustCompile(`<post\s+id=(\d+)\s*/>`)
	embeddedPostsRegex = regexp.MustCompile(`<posts\s+ids=(?:"([^"]+)"|(\S+))(?:\s+limit=(\d+))?\s*/>`)
)

func ParseEmbeddedTables(content string) []EmbeddedContent {
	tables := make([]EmbeddedContent, 0)

	// Parse <table ... /> embeds and extract type, filters,
	// and optional limit for each table embed
	tableMatches := embeddedTableRegex.FindAllStringSubmatch(content, -1)
	for _, match := range tableMatches {
		if len(match) >= 4 {
			filterStr := match[2]
			if filterStr == "" {
				filterStr = match[3]
			}
			limit := 100
			if len(match) > 4 && match[4] != "" {
				if l, err := strconv.Atoi(match[4]); err == nil {
					limit = l
				}
			}
			tables = append(tables, EmbeddedContent{
				Type:    match[1],
				Filters: parseFilters(filterStr),
				Limit:   limit,
			})
		}
	}

	// Parse <post ... /> embeds and extract the single post ID for each occurrence
	postMatches := embeddedPostRegex.FindAllStringSubmatch(content, -1)
	for _, match := range postMatches {
		if len(match) >= 2 {
			if id, err := strconv.Atoi(match[1]); err == nil {
				tables = append(tables, EmbeddedContent{
					Type:    "post",
					PostIDs: []int{id},
				})
			}
		}
	}

	// Parse <posts ... /> embeds and extract IDs and optional limit
	postsMatches := embeddedPostsRegex.FindAllStringSubmatch(content, -1)
	for _, match := range postsMatches {
		if len(match) >= 3 {
			idsStr := match[1]
			if idsStr == "" {
				idsStr = match[2]
			}
			ids := parsePostIDs(idsStr)
			if len(ids) > 0 {
				limit := len(ids)
				if len(match) > 3 && match[3] != "" {
					if l, err := strconv.Atoi(match[3]); err == nil {
						limit = l
					}
				}
				tables = append(tables, EmbeddedContent{
					Type:    "posts",
					PostIDs: ids,
					Limit:   limit,
				})
			}
		}
	}

	return tables
}

// parseFilters takes a string of filters, splits by whitespace,
// then parses key:value pairs and standalone tags into a map
func parseFilters(filterStr string) map[string]string {
	filters := make(map[string]string)
	parts := strings.Fields(filterStr)

	for _, part := range parts {
		if strings.Contains(part, ":") {
			kv := strings.SplitN(part, ":", 2)
			if len(kv) == 2 {
				filters[kv[0]] = kv[1]
			}
		} else {
			filters["tag"] = part
		}
	}

	return filters
}

// parsePostIDs parses a comma separated string of numeric post IDs
// into a slice of ints, if any part is not a valid integer, it's skipped
func parsePostIDs(idsStr string) []int {
	parts := strings.Split(idsStr, ",")
	ids := make([]int, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if id, err := strconv.Atoi(part); err == nil {
			ids = append(ids, id)
		}
	}

	return ids
}
