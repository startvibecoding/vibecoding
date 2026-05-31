// Package cron implements scheduled task management for vibecoding.
// Cron jobs are persisted to disk and executed by spawning sub-agents.
package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CronJob represents a scheduled task.
type CronJob struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`            // Short description
	Prompt     string    `json:"prompt"`           // Task prompt for sub-agent
	Schedule   string    `json:"schedule"`         // Schedule: @daily, @every 30m, 5-field cron, or empty for one-shot
	OneShot    bool      `json:"oneshot,omitempty"` // If true, auto-disable after first run
	Mode       string    `json:"mode"`             // "agent" or "yolo"
	WorkDir    string    `json:"work_dir,omitempty"`
	A2ATarget  string    `json:"a2a_target,omitempty"`  // A2A server URL (if set, send task via A2A protocol)
	A2AToken   string    `json:"a2a_token,omitempty"`   // Bearer token for A2A server
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	LastRun    time.Time `json:"last_run,omitempty"`
	NextRun    time.Time `json:"next_run,omitempty"`
	RunCount   int       `json:"run_count"`
	LastStatus string    `json:"last_status,omitempty"` // "success", "failed", "running"
	LastError  string    `json:"last_error,omitempty"`
}

// CronStore is the interface for cron job persistence.
type CronStore interface {
	List() ([]CronJob, error)
	Get(id string) (*CronJob, error)
	Create(job CronJob) (*CronJob, error)
	Update(job CronJob) error
	Delete(id string) error
}

// FileCronStore persists cron jobs to a JSON file.
type FileCronStore struct {
	mu       sync.RWMutex
	path     string
	jobs     map[string]*CronJob
}

// NewFileCronStore creates a new file-based cron store.
func NewFileCronStore(path string) *FileCronStore {
	s := &FileCronStore{
		path: path,
		jobs: make(map[string]*CronJob),
	}
	s.load()
	return s
}

func (s *FileCronStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return // File doesn't exist yet
	}
	var jobs []CronJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		return
	}
	for i := range jobs {
		s.jobs[jobs[i].ID] = &jobs[i]
	}
}

func (s *FileCronStore) save() error {
	jobs := make([]CronJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, *j)
	}
	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cron jobs: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create cron dir: %w", err)
	}
	return os.WriteFile(s.path, data, 0600)
}

// List returns all cron jobs.
func (s *FileCronStore) List() ([]CronJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]CronJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, *j)
	}
	return jobs, nil
}

// Get returns a cron job by ID.
func (s *FileCronStore) Get(id string) (*CronJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("cron job %q not found", id)
	}
	copy := *j
	return &copy, nil
}

// Create adds a new cron job.
func (s *FileCronStore) Create(job CronJob) (*CronJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job.ID == "" {
		job.ID = fmt.Sprintf("cron-%d", time.Now().UnixNano())
	}
	if _, exists := s.jobs[job.ID]; exists {
		return nil, fmt.Errorf("cron job %q already exists", job.ID)
	}
	job.CreatedAt = time.Now()
	copy := job
	s.jobs[job.ID] = &copy
	if err := s.save(); err != nil {
		delete(s.jobs, job.ID)
		return nil, err
	}
	return &copy, nil
}

// Update updates an existing cron job.
func (s *FileCronStore) Update(job CronJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[job.ID]; !ok {
		return fmt.Errorf("cron job %q not found", job.ID)
	}
	copy := job
	s.jobs[job.ID] = &copy
	return s.save()
}

// Delete removes a cron job.
func (s *FileCronStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[id]; !ok {
		return fmt.Errorf("cron job %q not found", id)
	}
	delete(s.jobs, id)
	return s.save()
}
