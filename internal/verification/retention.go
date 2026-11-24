package verification

import (
	"fmt"
	"sort"
	"time"
)

// Active jobs are never removed.
func (s *Service) ConfigureRetention(maxTerminal int, retention time.Duration) *Service {
	if maxTerminal > 0 {
		s.maxTerminal = maxTerminal
	}
	if retention > 0 {
		s.retention = retention
	}
	return s
}

func (s *Service) RetentionWarnings() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.cleanupWarnings...)
}

func (s *Service) pruneTerminalJobs() {
	now := time.Now().UTC()
	type candidate struct {
		id          string
		workspaceID string
		finished    time.Time
	}
	var remove []candidate
	s.mu.Lock()
	latestByWorkspace := map[string]string{}
	terminals := make([]candidate, 0)
	for id, job := range s.jobs {
		if job.Status != JobStatusPassed && job.Status != JobStatusFailed {
			continue
		}
		candidate := candidate{id: id, workspaceID: job.WorkspaceID, finished: job.FinishedAt}
		terminals = append(terminals, candidate)
		if previous, ok := latestByWorkspace[job.WorkspaceID]; !ok || s.jobs[previous].FinishedAt.Before(job.FinishedAt) {
			latestByWorkspace[job.WorkspaceID] = id
		}
	}
	sort.Slice(terminals, func(i, j int) bool { return terminals[i].finished.Before(terminals[j].finished) })
	remaining := len(terminals)
	for _, candidate := range terminals {
		expired := s.retention > 0 && !candidate.finished.IsZero() && now.Sub(candidate.finished) > s.retention
		overCapacity := remaining > s.maxTerminal
		if (expired || overCapacity) && latestByWorkspace[candidate.workspaceID] != candidate.id {
			delete(s.jobs, candidate.id)
			remove = append(remove, candidate)
			remaining--
		}
	}
	s.mu.Unlock()
	for _, candidate := range remove {
		if err := s.removeArtifactRoot(candidate.workspaceID, candidate.id); err != nil {
			s.mu.Lock()
			s.cleanupWarnings = append(s.cleanupWarnings, fmt.Sprintf("could not remove retained verification artifacts for %s: %v", candidate.id, err))
			s.mu.Unlock()
		}
	}
}

func (s *Service) recordCleanupWarning(message string) {
	s.mu.Lock()
	s.cleanupWarnings = append(s.cleanupWarnings, message)
	s.mu.Unlock()
}
