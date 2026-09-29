package validate

import "strings"

func ValidSubmissionID(value string) bool {
	return strings.TrimSpace(value) != "" && len(value) <= 128 && !strings.ContainsRune(value, '\x00')
}
