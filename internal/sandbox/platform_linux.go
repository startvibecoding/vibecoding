//go:build linux

package sandbox

// newPlatformSandbox creates the platform-specific sandbox for Linux.
func newPlatformSandbox(projectDir string, level Level) Sandbox {
	return NewBwrapSandbox(projectDir, level)
}
