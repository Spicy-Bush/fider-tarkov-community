package api

import (
	"errors"
	"strconv"
	"strings"
)

func parseRecordIDs(selection []string) ([]int, error) {
	if len(selection) != 1 {
		return nil, errors.New("invalid record selection")
	}

	values := strings.Split(selection[0], ",")
	if len(values) > 50 {
		return nil, errors.New("select at most 50 records")
	}

	ids := make([]int, len(values))
	for index, value := range values {
		id, err := strconv.ParseInt(value, 10, 32)
		if err != nil || id <= 0 {
			return nil, errors.New("invalid record ID")
		}

		ids[index] = int(id)
	}

	return ids, nil
}
