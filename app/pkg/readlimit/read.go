package readlimit

import (
	"errors"
	"io"
	"math"
)

var ErrTooLarge = errors.New("Content exceeds the requested byte limit")

func ReadAll(reader io.Reader, maximum int64) ([]byte, error) {
	if maximum <= 0 || maximum == math.MaxInt64 {
		return io.ReadAll(reader)
	}

	content, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return nil, err
	}

	if int64(len(content)) > maximum {
		return nil, ErrTooLarge
	}

	return content, nil
}
