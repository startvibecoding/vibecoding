package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.md")

	store := NewStore(path, "")

	// No file yet
	content, _, _, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if content != "" {
		t.Errorf("expected empty, got %q", content)
	}

	// Add creates file
	if err := store.Add("User Profile", "prefers Go"); err != nil {
		t.Fatal(err)
	}

	content, rpath, source, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if rpath != path {
		t.Errorf("expected path %s, got %s", path, rpath)
	}
	if source != "explicit" {
		t.Errorf("expected source explicit, got %s", source)
	}
	if !strings.Contains(content, "- prefers Go") {
		t.Errorf("expected content to contain 'prefers Go', got %q", content)
	}
}

func TestStoreReadSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.md")

	md := `# Agent Memory

## User Profile

- likes Go
- prefers vim

## Working Memory

- project version is v0.1.27

## Lessons Learned

- always read before edit
`
	os.WriteFile(path, []byte(md), 0600)
	store := NewStore(path, "")

	section, err := store.ReadSection("User Profile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(section, "likes Go") {
		t.Errorf("expected 'likes Go' in section, got %q", section)
	}
	if strings.Contains(section, "project version") {
		t.Error("section should not contain Working Memory content")
	}

	section, err = store.ReadSection("Working Memory")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(section, "project version") {
		t.Errorf("expected 'project version' in section, got %q", section)
	}

	section, err = store.ReadSection("Nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if section != "" {
		t.Errorf("expected empty for nonexistent section, got %q", section)
	}
}

func TestStoreAdd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.md")

	md := `# Agent Memory

## User Profile

- likes Go

## Working Memory
`
	os.WriteFile(path, []byte(md), 0600)
	store := NewStore(path, "")

	if err := store.Add("Working Memory", "new fact"); err != nil {
		t.Fatal(err)
	}

	content, _, _, _ := store.Read()
	if !strings.Contains(content, "- new fact") {
		t.Errorf("expected added entry, got %q", content)
	}
	// Original content should still be there
	if !strings.Contains(content, "- likes Go") {
		t.Errorf("original content lost")
	}
}

func TestStoreUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.md")

	md := `# Agent Memory

## Working Memory

- version is v0.1.26
`
	os.WriteFile(path, []byte(md), 0600)
	store := NewStore(path, "")

	if err := store.Update("Working Memory", "v0.1.26", "v0.1.27"); err != nil {
		t.Fatal(err)
	}

	content, _, _, _ := store.Read()
	if !strings.Contains(content, "v0.1.27") {
		t.Errorf("expected updated text, got %q", content)
	}
	if strings.Contains(content, "v0.1.26") {
		t.Error("old text should be replaced")
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.md")

	md := `# Agent Memory

## Working Memory

- fact one
- fact two
- fact three
`
	os.WriteFile(path, []byte(md), 0600)
	store := NewStore(path, "")

	if err := store.Delete("Working Memory", "fact two"); err != nil {
		t.Fatal(err)
	}

	content, _, _, _ := store.Read()
	if strings.Contains(content, "fact two") {
		t.Error("deleted entry should not be present")
	}
	if !strings.Contains(content, "fact one") || !strings.Contains(content, "fact three") {
		t.Error("non-deleted entries should remain")
	}
}

func TestStoreAddNewSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.md")

	md := `# Agent Memory

## User Profile

- likes Go
`
	os.WriteFile(path, []byte(md), 0600)
	store := NewStore(path, "")

	if err := store.Add("Custom Section", "custom fact"); err != nil {
		t.Fatal(err)
	}

	content, _, _, _ := store.Read()
	if !strings.Contains(content, "## Custom Section") {
		t.Error("new section should be created")
	}
	if !strings.Contains(content, "- custom fact") {
		t.Error("content should be added to new section")
	}
}

func TestExtractSection(t *testing.T) {
	content := `# Memory

## First

- a
- b

## Second

- c

## Third

- d
`
	first := extractSection(content, "First")
	if first != "- a\n- b" {
		t.Errorf("First section: %q", first)
	}

	second := extractSection(content, "Second")
	if second != "- c" {
		t.Errorf("Second section: %q", second)
	}

	third := extractSection(content, "Third")
	if third != "- d" {
		t.Errorf("Third section: %q", third)
	}

	missing := extractSection(content, "Missing")
	if missing != "" {
		t.Errorf("Missing section should be empty: %q", missing)
	}
}
