package config

import (
	"fmt"
	"strconv"
	"time"
)

// IdentityString returns the full app version string
func IdentityString() string {

	t, err := strconv.Atoi(Timestamp)
	timestamp := Timestamp
	if err == nil {
		timestamp = time.Unix(int64(t), 0).String()
	}
	return fmt.Sprintf(
		"%s %s (git: %s) - built at %s",
		Name,
		Version,
		GitSHA,
		timestamp,
	)
}
