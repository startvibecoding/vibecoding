package vendored

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// withTempHome sets HOME (or USERPROFILE on Windows) to a temp dir for the
// duration of the test so that binDir() / Ensure() / RgPath() / FdPath()
// don't touch the real ~/.vibecoding/bin/.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	} else {
		t.Setenv("HOME", dir)
	}
	return dir
}

// --- binDir ---

func TestBinDir(t *testing.T) {
	home := withTempHome(t)
	dir, err := binDir()
	if err != nil {
		t.Fatalf("binDir: %v", err)
	}
	want := filepath.Join(home, ".vibecoding", "bin")
	if dir != want {
		t.Errorf("binDir = %q, want %q", dir, want)
	}
}

// --- extractBinary ---

func TestExtractBinary_EmptyData(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "empty")
	err := extractBinary(dest, nil)
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestExtractBinary_WritesNew(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "bin")
	data := []byte("#!/bin/sh\necho hello\n")
	if err := extractBinary(dest, data); err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	// Verify file written
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != int64(len(data)) {
		t.Errorf("size = %d, want %d", info.Size(), len(data))
	}
	// Verify executable
	if runtime.GOOS != "windows" {
		if info.Mode()&0o111 == 0 {
			t.Error("file should be executable")
		}
	}
}

func TestExtractBinary_SkipsSameSize(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "bin")
	data := []byte("hello")

	// First write
	if err := extractBinary(dest, data); err != nil {
		t.Fatalf("first write: %v", err)
	}
	info1, _ := os.Stat(dest)
	modTime1 := info1.ModTime()

	// Second write — should skip (same size)
	if err := extractBinary(dest, data); err != nil {
		t.Fatalf("second write: %v", err)
	}
	info2, _ := os.Stat(dest)
	if info2.ModTime() != modTime1 {
		t.Error("file should not be rewritten when size matches")
	}
}

func TestExtractBinary_RewritesDifferentSize(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "bin")

	// Write v1
	if err := extractBinary(dest, []byte("v1")); err != nil {
		t.Fatalf("v1: %v", err)
	}
	// Write v2 (different size)
	v2 := []byte("version2")
	if err := extractBinary(dest, v2); err != nil {
		t.Fatalf("v2: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(v2) {
		t.Errorf("content = %q, want %q", got, v2)
	}
}

func TestExtractBinary_FixesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not applicable on Windows")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "bin")
	data := []byte("test")

	// Write file without execute permission
	os.WriteFile(dest, data, 0o644)

	// extractBinary should fix permissions
	if err := extractBinary(dest, data); err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	info, _ := os.Stat(dest)
	if info.Mode()&0o111 == 0 {
		t.Error("extractBinary should fix execute permission")
	}
}

// --- RgPath / FdPath ---

func TestRgPath(t *testing.T) {
	home := withTempHome(t)
	path := RgPath()
	if path == "" {
		t.Fatal("RgPath returned empty")
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	want := filepath.Join(home, ".vibecoding", "bin", "rg"+ext)
	if path != want {
		t.Errorf("RgPath = %q, want %q", path, want)
	}
}

func TestFdPath(t *testing.T) {
	home := withTempHome(t)
	path := FdPath()
	if path == "" {
		t.Fatal("FdPath returned empty")
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	want := filepath.Join(home, ".vibecoding", "bin", "fd"+ext)
	if path != want {
		t.Errorf("FdPath = %q, want %q", path, want)
	}
}

// --- Ensure ---

func TestEnsure(t *testing.T) {
	withTempHome(t)

	if err := Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	// Verify both binaries exist
	rgPath := RgPath()
	fdPath := FdPath()

	rgInfo, err := os.Stat(rgPath)
	if err != nil {
		t.Fatalf("rg not found at %s: %v", rgPath, err)
	}
	if rgInfo.Size() == 0 {
		t.Error("rg binary is empty")
	}

	fdInfo, err := os.Stat(fdPath)
	if err != nil {
		t.Fatalf("fd not found at %s: %v", fdPath, err)
	}
	if fdInfo.Size() == 0 {
		t.Error("fd binary is empty")
	}

	// Verify executable
	if runtime.GOOS != "windows" {
		if rgInfo.Mode()&0o111 == 0 {
			t.Error("rg should be executable")
		}
		if fdInfo.Mode()&0o111 == 0 {
			t.Error("fd should be executable")
		}
	}
}

func TestEnsure_Idempotent(t *testing.T) {
	withTempHome(t)

	// First call
	if err := Ensure(); err != nil {
		t.Fatalf("first Ensure: %v", err)
	}
	info1, _ := os.Stat(RgPath())

	// Second call — should skip (idempotent)
	if err := Ensure(); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	info2, _ := os.Stat(RgPath())

	if info2.ModTime() != info1.ModTime() {
		t.Error("Ensure should be idempotent (no rewrite on second call)")
	}
}
