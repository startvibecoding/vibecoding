package cron

import (
	"context"
	"testing"
)

func TestCronToolCreateOneShot(t *testing.T) {
	store := NewFileCronStore(t.TempDir() + "/cron.json")
	tool := NewCronTool(store, nil)

	result, err := tool.Execute(context.Background(), map[string]any{
		"action":  "create",
		"name":    "test-task",
		"prompt":  "do something",
		"oneshot": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text == "" {
		t.Error("expected non-empty result")
	}

	jobs, _ := store.List()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if !jobs[0].OneShot {
		t.Error("expected oneshot=true")
	}
	if jobs[0].Schedule != "" {
		t.Errorf("expected empty schedule, got %q", jobs[0].Schedule)
	}
}

func TestCronToolCreatePeriodic(t *testing.T) {
	store := NewFileCronStore(t.TempDir() + "/cron.json")
	tool := NewCronTool(store, nil)

	result, err := tool.Execute(context.Background(), map[string]any{
		"action":   "create",
		"name":     "daily-check",
		"prompt":   "check status",
		"schedule": "@daily",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text == "" {
		t.Error("expected non-empty result")
	}

	jobs, _ := store.List()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].OneShot {
		t.Error("expected oneshot=false for periodic")
	}
	if jobs[0].Schedule != "@daily" {
		t.Errorf("expected schedule @daily, got %q", jobs[0].Schedule)
	}
	if jobs[0].NextRun.IsZero() {
		t.Error("expected non-zero NextRun for periodic job")
	}
}

func TestCronToolCreateDefaultOneShot(t *testing.T) {
	store := NewFileCronStore(t.TempDir() + "/cron.json")
	tool := NewCronTool(store, nil)

	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "create",
		"name":   "default-task",
		"prompt": "do stuff",
		// no schedule, no oneshot → should default to one-shot
	})
	if err != nil {
		t.Fatal(err)
	}

	jobs, _ := store.List()
	if !jobs[0].OneShot {
		t.Error("expected default to be one-shot when no schedule")
	}
}

func TestCronToolList(t *testing.T) {
	store := NewFileCronStore(t.TempDir() + "/cron.json")
	tool := NewCronTool(store, nil)

	// Empty list
	result, _ := tool.Execute(context.Background(), map[string]any{"action": "list"})
	if result.Text != "No cron jobs configured." {
		t.Errorf("unexpected empty list: %s", result.Text)
	}

	// Add a job and list
	store.Create(CronJob{Name: "test", Prompt: "test", Enabled: true})
	result, _ = tool.Execute(context.Background(), map[string]any{"action": "list"})
	if result.Text == "No cron jobs configured." {
		t.Error("expected non-empty list")
	}
}

func TestCronToolEnableDisable(t *testing.T) {
	store := NewFileCronStore(t.TempDir() + "/cron.json")
	tool := NewCronTool(store, nil)

	job, _ := store.Create(CronJob{Name: "test", Prompt: "test", Enabled: true})

	// Disable
	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "disable",
		"id":     job.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	j, _ := store.Get(job.ID)
	if j.Enabled {
		t.Error("expected disabled")
	}

	// Enable
	_, err = tool.Execute(context.Background(), map[string]any{
		"action": "enable",
		"id":     job.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	j, _ = store.Get(job.ID)
	if !j.Enabled {
		t.Error("expected enabled")
	}
}

func TestCronToolRemove(t *testing.T) {
	store := NewFileCronStore(t.TempDir() + "/cron.json")
	tool := NewCronTool(store, nil)

	job, _ := store.Create(CronJob{Name: "test", Prompt: "test", Enabled: true})

	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "remove",
		"id":     job.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	jobs, _ := store.List()
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs after remove, got %d", len(jobs))
	}
}

func TestCronToolMissingParams(t *testing.T) {
	store := NewFileCronStore(t.TempDir() + "/cron.json")
	tool := NewCronTool(store, nil)

	// Create without name
	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "create",
		"prompt": "test",
	})
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Create without prompt
	_, err = tool.Execute(context.Background(), map[string]any{
		"action": "create",
		"name":   "test",
	})
	if err == nil {
		t.Error("expected error for missing prompt")
	}

	// Enable without id
	_, err = tool.Execute(context.Background(), map[string]any{
		"action": "enable",
	})
	if err == nil {
		t.Error("expected error for missing id")
	}
}

func TestCronToolUnknownAction(t *testing.T) {
	store := NewFileCronStore(t.TempDir() + "/cron.json")
	tool := NewCronTool(store, nil)

	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "invalid",
	})
	if err == nil {
		t.Error("expected error for unknown action")
	}
}
