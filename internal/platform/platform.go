// Package platform provides cross-platform compatibility utilities.
package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// OS returns the current operating system: "windows", "darwin", "linux", etc.
func OS() string {
	return runtime.GOOS
}

// IsWindows returns true if running on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsMacOS returns true if running on macOS.
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux returns true if running on Linux.
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// HomeDir returns the user's home directory.
func HomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// ConfigDir returns the platform-specific configuration directory.
func ConfigDir() string {
	if dir := os.Getenv("VIBECODING_DIR"); dir != "" {
		return dir
	}

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "vibecoding")
		}
		return filepath.Join(HomeDir(), "AppData", "Roaming", "vibecoding")
	case "darwin":
		return filepath.Join(HomeDir(), "Library", "Application Support", "vibecoding")
	default: // linux and others
		return filepath.Join(HomeDir(), ".vibecoding")
	}
}

// DataDir returns the platform-specific data directory.
func DataDir() string {
	return ConfigDir()
}

// CacheDir returns the platform-specific cache directory.
func CacheDir() string {
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "vibecoding", "cache")
		}
		return filepath.Join(HomeDir(), "AppData", "Local", "vibecoding", "cache")
	case "darwin":
		return filepath.Join(HomeDir(), "Library", "Caches", "vibecoding")
	default: // linux and others
		cacheHome := os.Getenv("XDG_CACHE_HOME")
		if cacheHome != "" {
			return filepath.Join(cacheHome, "vibecoding")
		}
		return filepath.Join(HomeDir(), ".cache", "vibecoding")
	}
}

// SessionDir returns the platform-specific session directory.
func SessionDir() string {
	return filepath.Join(ConfigDir(), "sessions")
}

// SkillsDir returns the platform-specific skills directory.
func SkillsDir() string {
	return filepath.Join(ConfigDir(), "skills")
}

// DefaultShell returns the default shell for the current platform.
func DefaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}

	switch runtime.GOOS {
	case "windows":
		// Try PowerShell first, then cmd
		if _, err := exec.LookPath("powershell.exe"); err == nil {
			return "powershell.exe"
		}
		return "cmd.exe"
	case "darwin":
		return "/bin/zsh"
	default: // linux and others
		return "/bin/bash"
	}
}

// ShellArgs returns the arguments to execute a command in the shell.
func ShellArgs(shell, command string) []string {
	normalizedShell := strings.ToLower(shell)
	switch {
	case strings.Contains(normalizedShell, "powershell"):
		return []string{"-NoProfile", "-NonInteractive", "-Command", command}
	case strings.Contains(normalizedShell, "cmd"):
		return []string{"/c", command}
	default: // bash, zsh, etc.
		return []string{"-c", command}
	}
}

// PathSeparator returns the platform-specific path separator.
func PathSeparator() string {
	return string(os.PathSeparator)
}

// JoinPath joins path elements using the platform-specific separator.
func JoinPath(elem ...string) string {
	return filepath.Join(elem...)
}

// NormalizePath normalizes a path for the current platform.
// Converts forward slashes to backslashes on Windows.
func NormalizePath(path string) string {
	return filepath.FromSlash(path)
}

// ExpandHome expands ~ to the user's home directory.
func ExpandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home := HomeDir()
	if home == "" {
		return path
	}

	if path == "~" {
		return home
	}

	if len(path) > 1 && (path[1] == '/' || path[1] == '\\') {
		return filepath.Join(home, path[2:])
	}

	return path
}

// CommonPaths returns platform-specific common system paths.
func CommonPaths() map[string]string {
	switch runtime.GOOS {
	case "windows":
		return map[string]string{
			"home":         HomeDir(),
			"temp":         os.TempDir(),
			"appData":      os.Getenv("APPDATA"),
			"localApp":     os.Getenv("LOCALAPPDATA"),
			"programFiles": os.Getenv("ProgramFiles"),
		}
	case "darwin":
		return map[string]string{
			"home":       HomeDir(),
			"temp":       os.TempDir(),
			"appSupport": filepath.Join(HomeDir(), "Library", "Application Support"),
			"caches":     filepath.Join(HomeDir(), "Library", "Caches"),
		}
	default: // linux
		return map[string]string{
			"home":   HomeDir(),
			"temp":   os.TempDir(),
			"cache":  filepath.Join(HomeDir(), ".cache"),
			"config": filepath.Join(HomeDir(), ".config"),
			"local":  filepath.Join(HomeDir(), ".local"),
		}
	}
}

// SandboxPaths returns paths that should be accessible in sandbox mode.
func SandboxPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			"C:\\Windows",
			"C:\\Program Files",
			"C:\\Program Files (x86)",
		}
	case "darwin":
		return []string{
			"/usr",
			"/lib",
			"/bin",
			"/sbin",
			"/System",
			"/Library",
		}
	default: // linux
		return []string{
			"/usr",
			"/lib",
			"/lib64",
			"/bin",
			"/sbin",
			"/etc/ld.so.cache",
			"/etc/ssl",
			"/etc/ca-certificates",
			"/dev/null",
			"/dev/urandom",
			"/dev/zero",
			"/proc/self",
			"/proc/meminfo",
			"/proc/cpuinfo",
		}
	}
}

// DeniedPaths returns paths that should be denied in sandbox mode.
func DeniedPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join(HomeDir(), "Documents"),
			filepath.Join(HomeDir(), "Desktop"),
		}
	default: // linux, darwin
		return []string{
			"/etc/shadow",
			"/etc/gshadow",
			"/etc/passwd",
			"/root",
			"/home",
		}
	}
}

// DefaultEnvVars returns environment variables to pass through sandbox.
func DefaultEnvVars() []string {
	common := []string{
		"PATH",
		"HOME",
		"USER",
		"LANG",
		"LC_ALL",
		"TERM",
	}

	switch runtime.GOOS {
	case "windows":
		return append(common,
			"APPDATA",
			"LOCALAPPDATA",
			"COMPUTERNAME",
			"USERPROFILE",
			"SYSTEMROOT",
		)
	case "darwin":
		return append(common,
			"SHELL",
			"TMPDIR",
		)
	default: // linux
		return append(common,
			"SHELL",
			"GOPATH",
			"GOROOT",
			"GOPROXY",
			"GOMODCACHE",
			"NODE_PATH",
		)
	}
}

// TempDir returns the platform-specific temp directory.
func TempDir() string {
	return os.TempDir()
}

// ExecutableExt returns the platform-specific executable extension.
func ExecutableExt() string {
	if IsWindows() {
		return ".exe"
	}
	return ""
}

// IsExecutable checks if a file is executable on the current platform.
func IsExecutable(info os.FileMode) bool {
	if IsWindows() {
		// On Windows, check file extension
		return true // Simplified for now
	}
	return info&0111 != 0
}
