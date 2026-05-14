//go:build windows

package sandbox

// newPlatformSandbox creates the platform-specific sandbox for Windows.
func newPlatformSandbox(projectDir string, level Level) Sandbox {
	return newWinSandbox(projectDir, level)
}
