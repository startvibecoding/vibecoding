package tools

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// BackgroundJob represents a running background process.
type BackgroundJob struct {
	ID        int
	Command   string
	PID       int
	StartTime time.Time
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	done      bool
	exitCode  int
	stdout    []byte
	stderr    []byte
	err       error
	mu        sync.Mutex
}

// JobManager manages background processes.
type JobManager struct {
	jobs     map[int]*BackgroundJob
	nextID   int
	mu       sync.RWMutex
	lastGC   time.Time // last time stale jobs were cleaned up
}

// NewJobManager creates a new job manager.
func NewJobManager() *JobManager {
	return &JobManager{
		jobs: make(map[int]*BackgroundJob),
	}
}

// AddJob adds a new background job.
func (jm *JobManager) AddJob(cmd *exec.Cmd, command string, cancel context.CancelFunc) *BackgroundJob {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	jm.gcStaleJobsLocked()

	jm.nextID++
	job := &BackgroundJob{
		ID:        jm.nextID,
		Command:   command,
		PID:       cmd.Process.Pid,
		StartTime: time.Now(),
		cmd:       cmd,
		cancel:    cancel,
	}
	jm.jobs[job.ID] = job
	return job
}

// GetJob returns a job by ID.
func (jm *JobManager) GetJob(id int) *BackgroundJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.jobs[id]
}

// ListJobs returns all jobs.
func (jm *JobManager) ListJobs() []*BackgroundJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	jobs := make([]*BackgroundJob, 0, len(jm.jobs))
	for _, j := range jm.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// KillJob kills a running job.
func (jm *JobManager) KillJob(id int) error {
	jm.mu.RLock()
	job, ok := jm.jobs[id]
	jm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("job %d not found", id)
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	if job.done {
		return fmt.Errorf("job %d already finished", id)
	}

	// Cancel the context to stop the process
	if job.cancel != nil {
		job.cancel()
	}

	if job.cmd.Process != nil {
		return job.cmd.Process.Kill()
	}
	return nil
}

// RemoveJob removes a finished job.
func (jm *JobManager) RemoveJob(id int) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	delete(jm.jobs, id)
}

// MarkDone marks a job as finished and stores output.
func (job *BackgroundJob) MarkDone(stdout, stderr []byte, err error) {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.done = true
	job.stdout = stdout
	job.stderr = stderr
	job.err = err
	if job.cmd.ProcessState != nil {
		job.exitCode = job.cmd.ProcessState.ExitCode()
	}
}

// IsDone returns whether the job is finished.
func (job *BackgroundJob) IsDone() bool {
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.done
}

// Status returns a string representation of the job status.
func (job *BackgroundJob) Status() string {
	job.mu.Lock()
	defer job.mu.Unlock()

	elapsed := time.Since(job.StartTime).Round(time.Second)
	if job.done {
		status := "finished"
		if job.exitCode != 0 {
			status = fmt.Sprintf("exited with code %d", job.exitCode)
		}
		return fmt.Sprintf("[%d] %s (PID: %d, %s, elapsed: %s)", job.ID, status, job.PID, job.Command, elapsed)
	}
	return fmt.Sprintf("[%d] running (PID: %d, %s, elapsed: %s)", job.ID, job.PID, job.Command, elapsed)
}

const staleJobTTL = 30 * time.Minute

// gcStaleJobsLocked removes finished jobs older than staleJobTTL.
// Caller must hold jm.mu.
func (jm *JobManager) gcStaleJobsLocked() {
	if time.Since(jm.lastGC) < 5*time.Minute {
		return
	}
	jm.lastGC = time.Now()
	for id, job := range jm.jobs {
		job.mu.Lock()
		stale := job.done && time.Since(job.StartTime) > staleJobTTL
		job.mu.Unlock()
		if stale {
			delete(jm.jobs, id)
		}
	}
}
