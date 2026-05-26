package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/startvibecoding/vibecoding/internal/agent"
)

// Scheduler checks for due cron jobs and executes them via sub-agents.
type Scheduler struct {
	store    CronStore
	manager  *agent.AgentManager
	interval time.Duration
	quit     chan struct{}
	running  bool
	mu       sync.Mutex
}

// NewScheduler creates a new cron scheduler.
func NewScheduler(store CronStore, manager *agent.AgentManager, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Scheduler{
		store:    store,
		manager:  manager,
		interval: interval,
		quit:     make(chan struct{}),
	}
}

// Start begins the scheduler loop.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.quit = make(chan struct{})
	s.mu.Unlock()

	go s.loop()
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.quit)
}

// IsRunning returns whether the scheduler is running.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Scheduler) loop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Check immediately on start
	s.checkAndRun()

	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
			s.checkAndRun()
		}
	}
}

// checkAndRun checks all enabled jobs and runs any that are due.
func (s *Scheduler) checkAndRun() {
	jobs, err := s.store.List()
	if err != nil {
		return
	}

	now := time.Now()
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		if job.LastStatus == "running" {
			continue // Don't start a job that's already running
		}
		if s.isDue(job, now) {
			go s.executeJob(job)
		}
	}
}

// isDue checks if a job should run now.
func (s *Scheduler) isDue(job CronJob, now time.Time) bool {
	// If never run, run now
	if job.LastRun.IsZero() {
		return true
	}
	// If NextRun is set and has passed
	if !job.NextRun.IsZero() && now.After(job.NextRun) {
		return true
	}
	// Simple interval-based fallback: run if last run was more than 1 hour ago
	if now.Sub(job.LastRun) > time.Hour {
		return true
	}
	return false
}

// executeJob runs a cron job by spawning a sub-agent.
func (s *Scheduler) executeJob(job CronJob) {
	// Mark as running
	job.LastStatus = "running"
	job.LastRun = time.Now()
	s.store.Update(job)

	a, err := s.manager.Create(agent.AgentOptions{
		Mode:    job.Mode,
		WorkDir: job.WorkDir,
	})
	if err != nil {
		job.LastStatus = "failed"
		job.LastError = fmt.Sprintf("create agent: %v", err)
		s.store.Update(job)
		return
	}

	ch := a.Run(context.Background(), job.Prompt)
	var lastErr error
	for event := range ch {
		if event.Error != nil {
			lastErr = event.Error
		}
	}

	job.RunCount++
	if lastErr != nil {
		job.LastStatus = "failed"
		job.LastError = lastErr.Error()
	} else {
		job.LastStatus = "success"
		job.LastError = ""
	}

	// Compute next run (simple: 1 hour from now)
	job.NextRun = time.Now().Add(time.Hour)

	s.store.Update(job)

	// Clean up the sub-agent
	s.manager.Destroy(a.ID())
}
