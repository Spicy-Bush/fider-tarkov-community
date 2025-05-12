package utils

import (
	"regexp"
	"strconv"
	"strings"
)

// ParseDuration converts a human-readable duration string to minutes.
func ParseDuration(input string) (int, bool) {
	if minutes, err := strconv.Atoi(input); err == nil {
		return minutes, true
	}

	pattern := regexp.MustCompile(`^(\d+)([mhdwMy])$`)
	matches := pattern.FindStringSubmatch(input)

	if len(matches) != 3 {
		return 0, false
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil || value < 0 {
		return 0, false
	}

	unit := matches[2]

	switch strings.ToLower(unit) {
	case "m":
		return value, true // minutes
	case "h":
		return value * 60, true // hours
	case "d":
		return value * 60 * 24, true // days
	case "w":
		return value * 60 * 24 * 7, true // weeks
	case "M":
		return value * 60 * 24 * 30, true // months (30 days)
	case "y":
		return value * 60 * 24 * 365, true // years
	default:
		return 0, false
	}
}
