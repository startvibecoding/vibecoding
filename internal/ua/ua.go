// Package ua provides User-Agent string generation for vibecoding.
package ua

import (
	"fmt"
	"runtime"
)

// Version is set at build time via ldflags.
var Version = "dev"

// UserAgent returns the User-Agent string for vibecoding.
// Format: vibecoding/{version} ({os}; go/{go_version}; {arch})
func UserAgent() string {
	return fmt.Sprintf("vibecoding/%s (%s; go/%s; %s)",
		Version,
		runtime.GOOS,
		runtime.Version()[2:], // Remove "go" prefix
		runtime.GOARCH,
	)
}

// ProviderUserAgent returns the User-Agent string for provider API calls.
// Format: vibecoding/{version} ({os}; go/{go_version}; {arch})
func ProviderUserAgent() string {
	return UserAgent()
}
