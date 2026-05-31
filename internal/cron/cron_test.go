package cron

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileCronStoreCreate(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	job, err := store.Create(CronJob{
		Name:     "test job",
		Prompt:   "do something",
		Schedule: "0 9 * * *",
		Mode:     "agent",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.ID == "" {
		t.Error("expected non-empty ID")
	}
	if job.Name != "test job" {
		t.Errorf("expected 'test job', got %q", job.Name)
	}
	if job.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestFileCronStoreCreateDuplicate(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	store.Create(CronJob{ID: "j1", Name: "first"})
	_, err := store.Create(CronJob{ID: "j1", Name: "duplicate"})
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

func TestFileCronStoreList(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	store.Create(CronJob{Name: "job1"})
	store.Create(CronJob{Name: "job2"})
	store.Create(CronJob{Name: "job3"})

	jobs, err := store.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(jobs))
	}
}

func TestFileCronStoreGet(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	created, _ := store.Create(CronJob{ID: "j1", Name: "test"})

	got, err := store.Get("j1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != created.Name {
		t.Errorf("expected %q, got %q", created.Name, got.Name)
	}
}

func TestFileCronStoreGetNotFound(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFileCronStoreUpdate(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	store.Create(CronJob{ID: "j1", Name: "original"})

	job, _ := store.Get("j1")
	job.Name = "updated"
	job.RunCount = 5
	if err := store.Update(*job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := store.Get("j1")
	if got.Name != "updated" {
		t.Errorf("expected 'updated', got %q", got.Name)
	}
	if got.RunCount != 5 {
		t.Errorf("expected RunCount=5, got %d", got.RunCount)
	}
}

func TestFileCronStoreUpdateNotFound(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	err := store.Update(CronJob{ID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFileCronStoreDelete(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	store.Create(CronJob{ID: "j1", Name: "to delete"})

	if err := store.Delete("j1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err := store.Get("j1")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestFileCronStoreDeleteNotFound(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	err := store.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFileCronStorePersistence(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cron.json")

	store1 := NewFileCronStore(path)
	store1.Create(CronJob{ID: "j1", Name: "persistent", Prompt: "test"})

	// Create a new store from the same file
	store2 := NewFileCronStore(path)
	got, err := store2.Get("j1")
	if err != nil {
		t.Fatalf("expected job to persist, got error: %v", err)
	}
	if got.Name != "persistent" {
		t.Errorf("expected 'persistent', got %q", got.Name)
	}
}

func TestFileCronStoreInvalidFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "invalid.json")
	os.WriteFile(path, []byte("not json"), 0600)

	// Should not panic, just return empty
	store := NewFileCronStore(path)
	jobs, _ := store.List()
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs from invalid file, got %d", len(jobs))
	}
}

// --- Scheduler tests ---

func TestSchedulerStartStop(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	// Create a mock manager (nil factory is ok for basic lifecycle tests)
	sched := NewScheduler(store, nil, 1*time.Second)

	if sched.IsRunning() {
		t.Error("expected not running initially")
	}

	sched.Start()
	if !sched.IsRunning() {
		t.Error("expected running after start")
	}

	// Double start should be no-op
	sched.Start()

	sched.Stop()
	if sched.IsRunning() {
		t.Error("expected not running after stop")
	}

	// Double stop should be no-op
	sched.Stop()
}

func TestSchedulerDefaultInterval(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))
	sched := NewScheduler(store, nil, 0)

	if sched.interval != 30*time.Second {
		t.Errorf("expected 30s default interval, got %v", sched.interval)
	}
}

func TestIsDueNeverRun(t *testing.T) {
	s := &Scheduler{}
	job := CronJob{Enabled: true}
	if !s.isDue(job, time.Now()) {
		t.Error("expected due for never-run job")
	}
}

func TestIsDueNextRunPassed(t *testing.T) {
	s := &Scheduler{}
	job := CronJob{
		Enabled: true,
		LastRun: time.Now().Add(-2 * time.Hour),
		NextRun: time.Now().Add(-1 * time.Hour),
	}
	if !s.isDue(job, time.Now()) {
		t.Error("expected due when NextRun has passed")
	}
}

func TestIsDueRecentRun(t *testing.T) {
	s := &Scheduler{}
	job := CronJob{
		Enabled: true,
		LastRun: time.Now().Add(-5 * time.Minute),
		NextRun: time.Now().Add(55 * time.Minute),
	}
	if s.isDue(job, time.Now()) {
		t.Error("expected not due for recent run with future NextRun")
	}
}

func TestIsDueOldRun(t *testing.T) {
	s := &Scheduler{}
	// A job with no NextRun and already run — should NOT be due (one-shot already done)
	job := CronJob{
		Enabled: true,
		LastRun: time.Now().Add(-2 * time.Hour),
	}
	if s.isDue(job, time.Now()) {
		t.Error("expected not due — no NextRun set, one-shot already completed")
	}

	// A job with NextRun in the past — should be due
	job2 := CronJob{
		Enabled: true,
		LastRun: time.Now().Add(-2 * time.Hour),
		NextRun: time.Now().Add(-30 * time.Minute),
	}
	if !s.isDue(job2, time.Now()) {
		t.Error("expected due — NextRun is in the past")
	}
}

func TestIsDueOneShotFirstRun(t *testing.T) {
	s := &Scheduler{}
	job := CronJob{
		Enabled:  true,
		OneShot:  true,
		LastRun:  time.Time{}, // never run
	}
	if !s.isDue(job, time.Now()) {
		t.Error("expected due — one-shot never run")
	}
}

func TestIsDuePeriodicJob(t *testing.T) {
	s := &Scheduler{}
	next := time.Now().Add(-5 * time.Minute) // 5 min ago
	job := CronJob{
		Enabled:  true,
		Schedule: "@hourly",
		LastRun:  time.Now().Add(-2 * time.Hour),
		NextRun:  next,
	}
	if !s.isDue(job, time.Now()) {
		t.Error("expected due — periodic job past NextRun")
	}
}

func TestIsDueDisabled(t *testing.T) {
	s := &Scheduler{}
	// isDue only checks timing; the checkAndRun loop skips disabled jobs.
	// But isDue itself should still return true for timing.
	job := CronJob{
		Enabled: false,
		LastRun: time.Time{}, // Never run
	}
	// isDue doesn't check Enabled flag — that's checked in checkAndRun.
	if !s.isDue(job, time.Now()) {
		t.Error("isDue should return true regardless of Enabled flag")
	}
}

func TestSchedulerCheckAndRunSkipsDisabledAndRunning(t *testing.T) {
	tmp := t.TempDir()
	store := NewFileCronStore(filepath.Join(tmp, "cron.json"))

	// Create disabled job
	store.Create(CronJob{ID: "disabled", Name: "Disabled", Enabled: false})

	// Create already running job
	runningJob := CronJob{ID: "running", Name: "Running", Enabled: true, LastStatus: "running"}
	store.Create(runningJob)

	sched := NewScheduler(store, nil, time.Second)
	// Should not panic even with nil manager (neither job should execute)
	sched.checkAndRun()

	// Verify no changes
	disabled, _ := store.Get("disabled")
	if disabled.LastStatus != "" {
		t.Errorf("disabled job status = %q, want empty", disabled.LastStatus)
	}
	running, _ := store.Get("running")
	if running.LastStatus != "running" {
		t.Errorf("running job status = %q, want 'running'", running.LastStatus)
	}
}

func TestCronJobStructFields(t *testing.T) {
	now := time.Now()
	job := CronJob{
		ID:         "j1",
		Name:       "Test Job",
		Prompt:     "Run tests",
		Schedule:   "0 9 * * *",
		Mode:       "agent",
		WorkDir:    "/home/user/project",
		Enabled:    true,
		CreatedAt:  now,
		LastRun:    now,
		NextRun:    now.Add(time.Hour),
		RunCount:   5,
		LastStatus: "success",
		LastError:  "",
	}

	if job.ID != "j1" {
		t.Errorf("ID = %q, want 'j1'", job.ID)
	}
	if job.RunCount != 5 {
		t.Errorf("RunCount = %d, want 5", job.RunCount)
	}
}
