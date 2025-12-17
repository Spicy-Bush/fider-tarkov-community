package web

import (
	"encoding/base64"
	"strconv"
	"strings"
)

type PostFilter struct {
	View       string
	Tags       []int
	Statuses   []int
	Limit      int
	Query      string
	MyVotes    bool
	MyPosts    bool
	NotMyVotes bool
}

var defaultFilter = PostFilter{
	View:  "trending",
	Limit: 20,
}

var viewNames = []string{
	"trending",
	"newest",
	"most-wanted",
	"most-discussed",
	"controversial",
	"recently-updated",
	"all",
}

// DecodePFilter decodes a compact bitmask-based filter cookie.
// Format: flags[:statusB64][:tagsB64]
// flags (decimal byte):
//   bits 0-2: view
//   bit 3: myVotes
//   bit 4: myPosts
//   bit 5: notMyVotes
func DecodePFilter(cookie string, isAuthenticated bool) PostFilter {
	if cookie == "" {
		pf := defaultFilter
		if isAuthenticated {
			pf.NotMyVotes = true
		}
		return pf
	}

	f := PostFilter{Limit: 20, View: "trending"}
	parts := strings.Split(cookie, ":")
	if len(parts) == 0 {
		if isAuthenticated {
			f.NotMyVotes = true
		}
		return f
	}

	if flags, err := strconv.Atoi(parts[0]); err == nil {
		view := flags & 0b00000111
		if view >= 0 && view < len(viewNames) {
			f.View = viewNames[view]
		}
		f.MyVotes = (flags & (1 << 3)) != 0
		f.MyPosts = (flags & (1 << 4)) != 0
		f.NotMyVotes = (flags & (1 << 5)) != 0
	}

	if len(parts) > 1 && parts[1] != "" {
		if bytes, err := base64.StdEncoding.DecodeString(parts[1]); err == nil && len(bytes) > 0 {
			f.Statuses = decodeBitmask(bytes)
		}
	}

	if len(parts) > 2 && parts[2] != "" {
		if bytes, err := base64.StdEncoding.DecodeString(parts[2]); err == nil && len(bytes) > 0 {
			f.Tags = decodeBitmask(bytes)
		}
	}

	return f
}

func decodeBitmask(bytes []byte) []int {
	count := 0
	for _, b := range bytes {
		count += popcount(b)
	}
	if count == 0 {
		return nil
	}

	ids := make([]int, 0, count)
	for i, b := range bytes {
		if b == 0 {
			continue
		}
		base := i << 3
		for bit := 0; bit < 8; bit++ {
			if b&(1<<bit) != 0 {
				ids = append(ids, base+bit)
			}
		}
	}
	return ids
}

func popcount(b byte) int {
	b = b - ((b >> 1) & 0x55)
	b = (b & 0x33) + ((b >> 2) & 0x33)
	return int((b + (b >> 4)) & 0x0F)
}
