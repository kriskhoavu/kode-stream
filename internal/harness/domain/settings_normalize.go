package domain

import "strings"

// Normalize fills defaults and validates every knob against its cap. It
// returns an InvalidError naming the first value out of range.
func (s Settings) Normalize() (Settings, error) {
	defaults := DefaultSettings()
	steps := []func() error{
		s.normalizePaused,
		s.normalizeSlots,
		func() error { return s.normalizeIntervals(defaults) },
		func() error { return s.normalizeRetention(defaults) },
		s.normalizeWorkspaces,
		s.normalizeModelPolicy,
		s.Ranker.normalize,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return s, err
		}
	}
	return s, nil
}

func (s *Settings) normalizePaused() error {
	if s.Paused == nil {
		s.Paused = map[RoleKey]bool{}
	}
	for key := range s.Paused {
		if !ValidRoleKey(key) {
			return Invalid("unknown role %q in paused", key)
		}
	}
	return nil
}

func (s *Settings) normalizeSlots() error {
	for _, pool := range []struct {
		name  string
		value *int
	}{{"planner", &s.Slots.Planner}, {"reviewer", &s.Slots.Reviewer}, {"coder", &s.Slots.Coder}, {"qa_fixer", &s.Slots.QAFixer}} {
		if *pool.value == 0 {
			*pool.value = 1
		}
		if *pool.value < 0 || *pool.value > maxSlotsPerPool {
			return Invalid("slots.%s must be between 1 and %d", pool.name, maxSlotsPerPool)
		}
	}
	return nil
}

func (s *Settings) normalizeIntervals(defaults Settings) error {
	for _, interval := range []struct {
		name     string
		value    *int
		fallback int
		min, max int
	}{
		{"scoutSeconds", &s.Intervals.ScoutSeconds, defaults.Intervals.ScoutSeconds, minIntervalSeconds, maxIntervalSeconds},
		{"sorterSeconds", &s.Intervals.SorterSeconds, defaults.Intervals.SorterSeconds, minIntervalSeconds, maxIntervalSeconds},
		{"workerSeconds", &s.Intervals.WorkerSeconds, defaults.Intervals.WorkerSeconds, 1, 3600},
		{"leaseSeconds", &s.Intervals.LeaseSeconds, defaults.Intervals.LeaseSeconds, minLeaseSeconds, maxLeaseSeconds},
	} {
		if *interval.value == 0 {
			*interval.value = interval.fallback
		}
		if *interval.value < interval.min || *interval.value > interval.max {
			return Invalid("intervals.%s must be between %d and %d", interval.name, interval.min, interval.max)
		}
	}
	return nil
}

func (s *Settings) normalizeRetention(defaults Settings) error {
	if s.RetentionDays == 0 {
		s.RetentionDays = defaults.RetentionDays
	}
	if s.RetentionDays < 1 || s.RetentionDays > maxRetentionDays {
		return Invalid("retentionDays must be between 1 and %d", maxRetentionDays)
	}
	return nil
}

func (s *Settings) normalizeWorkspaces() error {
	if s.Workspaces == nil {
		s.Workspaces = []WorkspaceSettings{}
	}
	seen := map[string]bool{}
	for i := range s.Workspaces {
		workspace := &s.Workspaces[i]
		workspace.WorkspaceID = strings.TrimSpace(workspace.WorkspaceID)
		if workspace.WorkspaceID == "" {
			return Invalid("workspaces[%d].workspaceId is required", i)
		}
		if seen[workspace.WorkspaceID] {
			return Invalid("workspace %s is listed twice", workspace.WorkspaceID)
		}
		seen[workspace.WorkspaceID] = true
		if err := workspace.normalize(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Settings) normalizeModelPolicy() error {
	policy, err := s.ModelPolicy.Normalize()
	if err != nil {
		return err
	}
	s.ModelPolicy = policy
	return nil
}

// normalize fills one workspace's defaults and checks its limits, its
// sources, its role models and its procedure.
func (w *WorkspaceSettings) normalize() error {
	for _, step := range []func() error{w.normalizeLimits, w.normalizeScope, w.normalizeJira, w.normalizeGitHub, w.normalizeBitbucket, w.normalizeLocal, w.normalizeModels, w.normalizeProcedure} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func (w *WorkspaceSettings) normalizeLimits() error {
	if w.MaxConcurrentRuns == 0 {
		w.MaxConcurrentRuns = 1
	}
	if w.MaxConcurrentRuns < 1 || w.MaxConcurrentRuns > maxConcurrentRuns {
		return Invalid("workspace %s maxConcurrentRuns must be between 1 and %d", w.WorkspaceID, maxConcurrentRuns)
	}
	if w.VerificationCommands == nil {
		w.VerificationCommands = []string{}
	}
	if len(w.VerificationCommands) > maxVerificationCmds {
		return Invalid("workspace %s lists more than %d verification commands", w.WorkspaceID, maxVerificationCmds)
	}
	for _, command := range w.VerificationCommands {
		if strings.TrimSpace(command) == "" || strings.ContainsAny(command, "\n\r") {
			return Invalid("workspace %s has an empty or multi-line verification command", w.WorkspaceID)
		}
	}
	return nil
}

func (w *WorkspaceSettings) normalizeScope() error {
	if w.Scope == nil {
		w.Scope = []string{}
	}
	for _, prefix := range w.Scope {
		clean := strings.TrimSpace(prefix)
		if clean == "" || strings.HasPrefix(clean, "/") || strings.Contains(clean, "..") {
			return Invalid("workspace %s scope entry %q must be a relative path inside the repository", w.WorkspaceID, prefix)
		}
	}
	return nil
}

func (w *WorkspaceSettings) normalizeJira() error {
	jira := w.Jira
	if jira == nil {
		return nil
	}
	if jira.Max == 0 {
		jira.Max = 50
	}
	if jira.Max < 1 || jira.Max > maxJiraResults {
		return Invalid("workspace %s jira.max must be between 1 and %d", w.WorkspaceID, maxJiraResults)
	}
	if jira.Enabled && strings.TrimSpace(jira.JQL) == "" {
		return Invalid("workspace %s jira.jql is required when Jira is enabled", w.WorkspaceID)
	}
	if len(jira.ReadyStatuses) > MaxReadyStatuses {
		return Invalid("workspace %s names more than %d ready statuses", w.WorkspaceID, MaxReadyStatuses)
	}
	statuses := []string{}
	for _, status := range jira.ReadyStatuses {
		status = strings.TrimSpace(status)
		if status == "" {
			continue
		}
		if len(status) > 64 {
			return Invalid("workspace %s ready status %q is longer than 64 characters", w.WorkspaceID, status)
		}
		statuses = append(statuses, status)
	}
	jira.ReadyStatuses = statuses
	return nil
}

func (w *WorkspaceSettings) normalizeGitHub() error {
	github := w.GitHub
	if github == nil {
		return nil
	}
	if github.Labels == nil {
		github.Labels = []string{}
	}
	if github.Enabled && strings.Count(strings.TrimSpace(github.Repo), "/") != 1 {
		return Invalid("workspace %s github.repo must be owner/name", w.WorkspaceID)
	}
	if github.TokenEnvVar == "" {
		github.TokenEnvVar = "GITHUB_TOKEN"
	}
	return nil
}

func (w *WorkspaceSettings) normalizeBitbucket() error {
	bitbucket := w.Bitbucket
	if bitbucket == nil {
		return nil
	}
	if bitbucket.Enabled && (strings.TrimSpace(bitbucket.Workspace) == "" || strings.TrimSpace(bitbucket.Repo) == "") {
		return Invalid("workspace %s bitbucket.workspace and bitbucket.repo are required", w.WorkspaceID)
	}
	if bitbucket.TokenEnvVar == "" {
		bitbucket.TokenEnvVar = "BITBUCKET_TOKEN"
	}
	return nil
}

func (w *WorkspaceSettings) normalizeLocal() error {
	if w.Local != nil && w.Local.Enabled && strings.TrimSpace(w.Local.Dir) == "" {
		return Invalid("workspace %s local.dir is required when the local source is enabled", w.WorkspaceID)
	}
	return nil
}

func (w *WorkspaceSettings) normalizeModels() error {
	for role, model := range w.Models {
		if !validRole(role) {
			return Invalid("workspace %s names an unknown role %q in models", w.WorkspaceID, role)
		}
		if model.MaxTurns < 0 || model.MaxTurns > DefaultPolicyMaxTurn {
			return Invalid("workspace %s models.%s.maxTurns must be between 1 and %d", w.WorkspaceID, role, DefaultPolicyMaxTurn)
		}
		if model == (RoleModel{}) {
			delete(w.Models, role)
		}
	}
	return nil
}

func (w *WorkspaceSettings) normalizeProcedure() error {
	for role, step := range w.Procedure {
		if err := ValidateProcedure(role, step); err != nil {
			return Invalid("workspace %s: %v", w.WorkspaceID, err)
		}
	}
	return nil
}

func (r *RankerSettings) normalize() error {
	if r.LabelBoosts == nil {
		r.LabelBoosts = map[string]float64{}
	}
	for label, boost := range r.LabelBoosts {
		if boost < -100 || boost > 100 {
			return Invalid("ranker.labelBoosts[%s] must be between -100 and 100", label)
		}
	}
	if r.LLM.TopN == 0 {
		r.LLM.TopN = 10
	}
	if r.LLM.TopN < 2 || r.LLM.TopN > 50 {
		return Invalid("ranker.llm.topN must be between 2 and 50")
	}
	if r.LLM.Enabled && (strings.TrimSpace(r.LLM.Endpoint) == "" || strings.TrimSpace(r.LLM.Model) == "") {
		return Invalid("ranker.llm.endpoint and ranker.llm.model are required when the re-ranker is enabled")
	}
	return nil
}
