package cron

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

var a2aHTTPClient = &http.Client{Timeout: 30 * time.Second}

const maxA2AResponseBytes = 1 << 20

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
		log.Printf("[cron] failed to list jobs: %v", err)
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
	return false
}

// executeJob runs a cron job by spawning a sub-agent or sending to A2A server.
func (s *Scheduler) executeJob(job CronJob) {
	// Mark as running
	startedAt := time.Now()
	s.updateJob(job.ID, func(current *CronJob) {
		current.LastStatus = "running"
		current.LastRun = startedAt
	})

	var lastErr error

	// A2A target mode: send task to remote A2A server
	if job.A2ATarget != "" {
		lastErr = s.executeA2AJob(job)
	} else {
		// Local agent mode
		a, err := s.manager.Create(agent.AgentOptions{
			Mode:    job.Mode,
			WorkDir: job.WorkDir,
		})
		if err != nil {
			s.updateJob(job.ID, func(current *CronJob) {
				current.LastStatus = "failed"
				current.LastError = fmt.Sprintf("create agent: %v", err)
			})
			return
		}

		ch := a.Run(context.Background(), job.Prompt)
		for event := range ch {
			if event.Error != nil {
				lastErr = event.Error
			}
		}
		s.manager.Destroy(a.ID())
	}

	s.updateJob(job.ID, func(current *CronJob) {
		current.RunCount++
		if lastErr != nil {
			current.LastStatus = "failed"
			current.LastError = lastErr.Error()
		} else {
			current.LastStatus = "success"
			current.LastError = ""
		}

		// Compute next run from the latest stored schedule.
		next, isOneShot, err := ParseSchedule(current.Schedule, time.Now())
		if err != nil {
			isOneShot = true
		}
		if isOneShot || current.OneShot {
			current.Enabled = false
			current.NextRun = time.Time{}
		} else {
			current.NextRun = next
		}
	})
}

func (s *Scheduler) updateJob(id string, update func(*CronJob)) {
	current, err := s.store.Get(id)
	if err != nil {
		return
	}
	update(current)
	_ = s.store.Update(*current)
}

// executeA2AJob sends a task to a remote A2A server.
func (s *Scheduler) executeA2AJob(job CronJob) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "message/send",
		"params": map[string]any{
			"message": map[string]any{
				"role":  "user",
				"parts": []map[string]string{{"type": "text", "text": job.Prompt}},
			},
		},
		"id": 1,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", job.A2ATarget+"/a2a", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if job.A2AToken != "" {
		req.Header.Set("Authorization", "Bearer "+job.A2AToken)
	}

	resp, err := a2aHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("a2a request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("a2a request: status %d", resp.StatusCode)
	}

	var result struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxA2AResponseBytes)).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if result.Error != nil {
		return fmt.Errorf("a2a error: %s", result.Error.Message)
	}
	return nil
}
