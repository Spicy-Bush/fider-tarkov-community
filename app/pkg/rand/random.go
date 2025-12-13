package rand

import (
	"crypto/rand"
)

var chars = []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// String returns a random string of given length
// uses a single syscall to read all random bytes, then maps to charset
func String(n int) string {
	if n <= 0 {
		return ""
	}

	buf := make([]byte, n)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	for i := range buf {
		buf[i] = chars[buf[i]%byte(len(chars))]
	}

	return string(buf)
}
