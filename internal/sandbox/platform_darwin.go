//go:build darwin

package sandbox

// newPlatformSandbox creates the platform-specific sandbox for macOS.
func newPlatformSandbox(projectDir string, level Level) Sandbox {
	return newMacSandbox(projectDir, level)
}
