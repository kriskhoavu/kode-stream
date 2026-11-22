package verification

import (
	"fmt"
	"strconv"
	"time"
)

func (s *Service) failJob(jobID string, failure FailureType, code int, message string, err error) {
	s.mutate(jobID, func(job *Job) {
		job.FailureType = failure
		job.ExitCode = code
		job.Steps = append(job.Steps, StepResult{
			Step:       "failure",
			Status:     "failed",
			Message:    fmt.Sprintf("%s: %v", message, err),
			DurationMS: 0,
			At:         time.Now().UTC(),
		})
	})
}

func (s *Service) snapshot(workspaceID, jobID string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	if !ok || job.WorkspaceID != workspaceID {
		return Job{}, false
	}
	return cloneJob(*job), true
}

func (s *Service) snapshotAny(jobID string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return Job{}, false
	}
	return cloneJob(*job), true
}

func (s *Service) mutate(jobID string, mutate func(*Job)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[jobID]; ok {
		mutate(job)
	}
}

func (s *Service) nextID() string {
	n := s.seq.Add(1)
	return "verify-" + strconv.FormatInt(time.Now().UTC().Unix(), 10) + "-" + strconv.FormatInt(n, 10)
}

func cloneJob(job Job) Job {
	copy := job
	copy.Steps = append([]StepResult(nil), job.Steps...)
	copy.Artifacts = append([]Artifact(nil), job.Artifacts...)
	copy.SelectedSpecs = append([]string(nil), job.SelectedSpecs...)
	return copy
}
