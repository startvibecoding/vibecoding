// Package ua provides User-Agent string generation for vibecoding.
package ua

import (
	"fmt"
	"runtime"
)

// Version is set at build time via ldflags.
var Version = "dev"

// UserAgent returns the User-Agent string for vibecoding.
// Format: Claude-User (vibecoding/{version}; +https://github.com/fuckvibecoding/vibecoding)
func UserAgent() string {
	return fmt.Sprintf("Claude-User (vibecoding/%s; +https://github.com/fuckvibecoding/vibecoding)",
		Version,
	)
}

// ProviderUserAgent returns the User-Agent string for provider API calls.
func ProviderUserAgent() string {
	return UserAgent()
}

// DetailedUserAgent returns a more detailed User-Agent string with OS info.
func DetailedUserAgent() string {
	return fmt.Sprintf("Claude-User (vibecoding/%s; %s; %s; +https://github.com/fuckvibecoding/vibecoding)",
		Version,
		runtime.GOOS,
		runtime.GOARCH,
	)
}
