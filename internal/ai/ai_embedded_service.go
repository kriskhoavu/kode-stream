package ai

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/pathguard"
)

// embeddedSessionID names a new embedded session and returns the arguments it
// launches with. A Claude Code session is named after its conversation: the
// record ID carries the conversation's UUID, passed to Claude as --session-id,
// so the conversation can be told apart from the user's own and resumed later.
func embeddedSessionID(providerID string, provider LaunchTemplate, args []string) (string, []string) {
	if !isClaudeProvider(providerID, provider) {
		return "embedded-" + randomID(), args
	}
	conversation := uuid.NewString()
	return "embedded-" + conversation, append([]string{"--session-id", conversation}, args...)
}

func isClaudeProvider(providerID string, provider LaunchTemplate) bool {
	return providerID == "claude" || strings.TrimSuffix(filepath.Base(provider.Executable), ".exe") == "claude"
}

type EmbeddedInput struct {
	Provider            string   `json:"provider"`
	ContextMode         string   `json:"contextMode"`
	PresetID            string   `json:"presetId,omitempty"`
	PromptDraft         string   `json:"promptDraft,omitempty"`
	CustomPrompt        string   `json:"customPrompt,omitempty"`
	SelectedSkills      []string `json:"selectedSkills,omitempty"`
	SelectedAgents      []string `json:"selectedAgents,omitempty"`
	ContextPath         string   `json:"contextPath,omitempty"`
	ExpectedWorkspaceID string   `json:"expectedWorkspaceId,omitempty"`
	ExpectedBranch      string   `json:"expectedBranch,omitempty"`
	ObservedCommit      string   `json:"observedCommit,omitempty"`
	IdempotencyKey      string   `json:"idempotencyKey,omitempty"`
	// ResumeClaudeSessionID resumes one of the workspace's Claude conversations
	// instead of starting a new one; the prompt inputs are then ignored.
	ResumeClaudeSessionID string `json:"resumeClaudeSessionId,omitempty"`
	Columns               uint16 `json:"columns"`
	Rows                  uint16 `json:"rows"`
}

func (s *Service) StartEmbeddedWorkspace(workspaceID string, input EmbeddedInput) (EmbeddedResult, error) {
	if s.embedded == nil || s.launch == nil || s.launch.registry == nil {
		return EmbeddedResult{}, launchError("launch_failed", "embedded AI sessions are unavailable")
	}
	s.embeddedLaunchMu.Lock()
	defer s.embeddedLaunchMu.Unlock()
	workspace, found, err := s.launch.registry.Get(workspaceID)
	if err != nil {
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return EmbeddedResult{}, launchError("workspace_not_found", "workspace not found")
	}
	if expected := strings.TrimSpace(input.ExpectedWorkspaceID); expected != "" && expected != workspace.ID {
		return EmbeddedResult{}, launchErrorWithDetails("terminal_workspace_mismatch", "the requested workspace no longer matches the selected workspace", map[string]string{"expectedWorkspaceId": expected, "currentWorkspaceId": workspace.ID})
	}
	if result, found, err := s.existingSession(workspace.ID, input.IdempotencyKey); err != nil || found {
		return result, err
	}
	branch, commit, err := s.validateSessionBranch(workspace, input.ExpectedBranch, input.ObservedCommit)
	if err != nil {
		return EmbeddedResult{}, err
	}
	contextPath := strings.TrimSpace(input.ContextPath)
	if contextPath != "" {
		if _, err := pathguard.ValidateMarkdownFile(workspace.Path, contextPath); err != nil {
			return EmbeddedResult{}, launchError("invalid_context_path", "context path must be an existing Markdown file inside the workspace")
		}
	}
	providerID := strings.TrimSpace(input.Provider)
	provider, capability, err := s.embeddedProvider(providerID)
	if err != nil {
		return EmbeddedResult{}, err
	}
	prompt, _, err := s.composeWorkspacePrompt(providerID, workspaceID, "workspace_only", input.PresetID, input.PromptDraft, input.CustomPrompt, input.SelectedSkills, input.SelectedAgents)
	if err != nil {
		return EmbeddedResult{}, err
	}
	contextLabel := contextPath
	if contextLabel == "" {
		contextLabel = "workspace root"
	}
	values := map[string]string{"workspace": workspace.Path, "contextFile": contextLabel, "itemPath": contextLabel, "identifier": contextLabel, "contextMode": "workspace_only", "intent": "workspace_only", "prompt": prompt}
	recordID, args := embeddedSessionID(providerID, provider, launchProviderArgs("workspace_only", provider.Args, values))
	now := s.sessionNow()
	record, err := s.persistStartingSession(SessionRecord{ID: recordID, WorkspaceID: workspace.ID, Provider: providerID, Intent: "workspace_only", RequestedBranch: branch, ObservedCommit: commit, IdempotencyKey: input.IdempotencyKey, State: StateStarting, StartedAt: now, LastKnownAt: now})
	if err != nil {
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	return s.startRecorded(record, StartRequest{ID: record.ID, WorkspaceID: workspace.ID, Provider: providerID, Intent: "workspace_only", Executable: capability.Executable, Args: args, Dir: workspace.Path, Columns: input.Columns, Rows: input.Rows})
}

type EmbeddedResult struct {
	Session Session        `json:"session"`
	Grant   Grant          `json:"grant"`
	Record  *SessionRecord `json:"record,omitempty"`
}

func (s *Service) StartEmbedded(itemID string, input EmbeddedInput) (EmbeddedResult, error) {
	if s.embedded == nil || s.launch == nil || s.launch.registry == nil || s.launch.index == nil {
		return EmbeddedResult{}, launchError("launch_failed", "embedded AI sessions are unavailable")
	}
	s.embeddedLaunchMu.Lock()
	defer s.embeddedLaunchMu.Unlock()
	item, workspace, err := s.embeddedTarget(itemID, input.ExpectedWorkspaceID)
	if err != nil {
		return EmbeddedResult{}, err
	}
	if result, found, err := s.existingSession(workspace.ID, input.IdempotencyKey); err != nil || found {
		return result, err
	}
	requestedBranch, observedCommit, err := embeddedRevision(item, input)
	if err != nil {
		return EmbeddedResult{}, err
	}
	branch, commit, err := s.validateSessionBranch(workspace, requestedBranch, observedCommit)
	if err != nil {
		return EmbeddedResult{}, err
	}
	resume := strings.TrimSpace(input.ResumeClaudeSessionID)
	if resume != "" {
		return s.resumeClaudeEmbedded(itemID, item, workspace, branch, commit, resume, input)
	}
	mode := strings.TrimSpace(input.ContextMode)
	if err := checkEmbeddedContext(mode, item, workspace); err != nil {
		return EmbeddedResult{}, err
	}
	providerID := strings.TrimSpace(input.Provider)
	provider, capability, err := s.embeddedProvider(providerID)
	if err != nil {
		return EmbeddedResult{}, err
	}
	prompt, _, promptErr := s.composePrompt(input.Provider, itemID, input.ContextMode, input.PresetID, input.PromptDraft, input.CustomPrompt, input.SelectedSkills, input.SelectedAgents)
	if promptErr != nil {
		return EmbeddedResult{}, promptErr
	}
	values := map[string]string{"workspace": workspace.Path, "contextFile": item.ItemPath, "itemPath": item.ItemPath, "identifier": item.Identifier, "contextMode": mode, "intent": mode, "prompt": prompt}
	recordID, args := embeddedSessionID(providerID, provider, launchProviderArgs(mode, provider.Args, values))
	now := s.sessionNow()
	record, err := s.persistStartingSession(SessionRecord{ID: recordID, WorkspaceID: workspace.ID, PlanRef: &SessionPlanRef{ItemID: itemID, ItemPath: item.ItemPath, Identifier: item.Identifier, BranchKey: branch, ObservedCommit: commit}, Provider: providerID, Intent: mode, RequestedBranch: branch, ObservedCommit: commit, IdempotencyKey: input.IdempotencyKey, State: StateStarting, StartedAt: now, LastKnownAt: now})
	if err != nil {
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	return s.startRecorded(record, StartRequest{ID: record.ID, ItemID: itemID, ItemIdentifier: item.Identifier, ItemTitle: item.Title, WorkspaceID: item.WorkspaceID, Provider: providerID, Intent: mode, Executable: capability.Executable, Args: args, Dir: workspace.Path, Columns: input.Columns, Rows: input.Rows})
}

// resumeClaudeEmbedded continues one of the workspace's Claude conversations
// in an embedded terminal for the item. The record is named after the
// conversation with a suffix, since one conversation may be resumed many times.
func (s *Service) resumeClaudeEmbedded(itemID string, item models.ItemDetail, workspace models.WorkspaceConfig, branch, commit, conversation string, input EmbeddedInput) (EmbeddedResult, error) {
	if _, err := s.claudeTranscriptPath(workspace.ID, conversation); err != nil {
		if errors.Is(err, ErrNotFound) {
			return EmbeddedResult{}, launchError("claude_session_not_found", "the Claude conversation is not held in this workspace")
		}
		var launchErr *LaunchError
		if errors.As(err, &launchErr) {
			return EmbeddedResult{}, err
		}
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	settings, err := s.Settings()
	if err != nil {
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	providerID := strings.TrimSpace(input.Provider)
	if providerID == "" {
		providerID = "claude"
	}
	provider, ok := settings.Providers[providerID]
	if !ok || !provider.Enabled || !isClaudeProvider(providerID, provider) {
		return EmbeddedResult{}, launchError("ai_provider_missing", "the Claude provider is unavailable")
	}
	capability := s.detect(provider.Executable)
	if !capability.Detected {
		return EmbeddedResult{}, launchError("ai_provider_missing", "the Claude executable was not found")
	}
	const intent = "resume"
	now := s.sessionNow()
	record, err := s.persistStartingSession(SessionRecord{ID: "embedded-" + conversation + "-" + randomID(), WorkspaceID: workspace.ID, PlanRef: &SessionPlanRef{ItemID: itemID, ItemPath: item.ItemPath, Identifier: item.Identifier, BranchKey: branch, ObservedCommit: commit}, Provider: providerID, Intent: intent, RequestedBranch: branch, ObservedCommit: commit, IdempotencyKey: input.IdempotencyKey, State: StateStarting, StartedAt: now, LastKnownAt: now})
	if err != nil {
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	return s.startRecorded(record, StartRequest{ID: record.ID, ItemID: itemID, ItemIdentifier: item.Identifier, ItemTitle: item.Title, WorkspaceID: item.WorkspaceID, Provider: providerID, Intent: intent, Executable: capability.Executable, Args: []string{"--resume", conversation}, Dir: workspace.Path, Columns: input.Columns, Rows: input.Rows})
}

// embeddedTarget finds the item and its workspace, and refuses an item that
// moved to a workspace other than the one the caller saw.
func (s *Service) embeddedTarget(itemID, expectedWorkspaceID string) (models.ItemDetail, models.WorkspaceConfig, error) {
	item, found, err := s.launch.index.Get(itemID)
	if err != nil {
		return models.ItemDetail{}, models.WorkspaceConfig{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return models.ItemDetail{}, models.WorkspaceConfig{}, launchError("item_not_found", "item not found")
	}
	workspace, found, err := s.launch.registry.Get(item.WorkspaceID)
	if err != nil {
		return models.ItemDetail{}, models.WorkspaceConfig{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return models.ItemDetail{}, models.WorkspaceConfig{}, launchError("workspace_not_found", "workspace not found")
	}
	if expected := strings.TrimSpace(expectedWorkspaceID); expected != "" && expected != item.WorkspaceID {
		return models.ItemDetail{}, models.WorkspaceConfig{}, launchErrorWithDetails("terminal_workspace_mismatch", "the plan reference moved to a different workspace", map[string]string{"expectedWorkspaceId": expected, "currentWorkspaceId": item.WorkspaceID})
	}
	return item, workspace, nil
}

// embeddedRevision picks the branch and commit a session starts from, and
// refuses an item that moved to a branch other than the one the caller saw.
func embeddedRevision(item models.ItemDetail, input EmbeddedInput) (string, string, error) {
	if expected := strings.TrimSpace(input.ExpectedBranch); expected != "" && strings.TrimSpace(item.Branch) != "" && expected != item.Branch {
		return "", "", launchErrorWithDetails("terminal_branch_mismatch", "the plan reference moved to a different branch", map[string]string{"expectedBranch": expected, "currentBranch": item.Branch})
	}
	requestedBranch := strings.TrimSpace(item.Branch)
	if requestedBranch == "" {
		requestedBranch = strings.TrimSpace(input.ExpectedBranch)
	}
	observedCommit := strings.TrimSpace(input.ObservedCommit)
	if observedCommit == "" {
		observedCommit = strings.TrimSpace(item.Commit)
	}
	return requestedBranch, observedCommit, nil
}

// checkEmbeddedContext accepts the two context modes, and card context only
// for an editable working-tree item inside its workspace.
func checkEmbeddedContext(mode string, item models.ItemDetail, workspace models.WorkspaceConfig) error {
	if mode != "workspace_only" && mode != "card_context" {
		return launchError("invalid_context_mode", "contextMode must be workspace_only or card_context")
	}
	if mode != "card_context" {
		return nil
	}
	if item.SourceMode == "snapshot" || !item.Editable {
		return launchError("item_not_editable", "context-based AI sessions require an editable working-tree item")
	}
	if _, err := pathguard.SafeJoin(workspace.Path, item.ItemPath); err != nil {
		return launchError("item_not_editable", "item path is outside the workspace")
	}
	return nil
}

// embeddedProvider returns an enabled provider whose executable is installed.
func (s *Service) embeddedProvider(providerID string) (LaunchTemplate, Capability, error) {
	settings, err := s.Settings()
	if err != nil {
		return LaunchTemplate{}, Capability{}, launchErrorWith("launch_failed", err)
	}
	provider, ok := settings.Providers[providerID]
	if !ok || !provider.Enabled {
		return LaunchTemplate{}, Capability{}, launchError("ai_provider_missing", "selected AI provider is unavailable")
	}
	capability := s.detect(provider.Executable)
	if !capability.Detected {
		return LaunchTemplate{}, Capability{}, launchError("ai_provider_missing", "selected AI provider executable was not found")
	}
	return provider, capability, nil
}

// startRecorded starts the terminal for a persisted starting record. When the
// start fails it marks the record failed, so no record stays starting.
func (s *Service) startRecorded(record SessionRecord, request StartRequest) (EmbeddedResult, error) {
	session, grant, err := s.embedded.Start(request)
	if err != nil {
		if compensationErr := s.markLaunchFailed(record); compensationErr != nil {
			return EmbeddedResult{}, launchErrorWith("launch_failed", fmt.Errorf("%w; session compensation failed: %v", err, compensationErr))
		}
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	if saved, ok, getErr := s.recordsGet(record.ID); getErr == nil && ok {
		record = saved
	}
	return EmbeddedResult{Session: session, Grant: grant, Record: &record}, nil
}

func (s *Service) markLaunchFailed(record SessionRecord) error {
	if s.records == nil {
		return nil
	}
	s.sessionRecordMu.Lock()
	defer s.sessionRecordMu.Unlock()
	now := s.sessionNow()
	record.State = StateFailed
	record.EndedAt = now
	record.LastKnownAt = now
	// A failed reservation must not occupy the caller's idempotency key: there
	// is no live process for a retry to discover.
	record.IdempotencyKey = ""
	_, err := s.records.Upsert(record)
	return err
}

func (s *Service) recordsGet(id string) (SessionRecord, bool, error) {
	if s.records == nil {
		return SessionRecord{}, false, nil
	}
	return s.records.Get(id)
}

// ReviewIntent marks a session a person opened at a harness run's Review
// stage. Its branch is the run's own branch, which names the run.
const ReviewIntent = "harness_review"

// ReviewInput starts a session in a harness run's worktree. The harness
// supplies the folder, branch and brief; the caller only picks a provider.
type ReviewInput struct {
	WorkspaceID    string
	Dir            string
	Branch         string
	Prompt         string
	Provider       string
	IdempotencyKey string
	Columns        uint16
	Rows           uint16
}

// StartEmbeddedReview starts an embedded session in a harness run's
// worktree, so a person can try the change and edit it on the run's branch.
func (s *Service) StartEmbeddedReview(input ReviewInput) (EmbeddedResult, error) {
	if s.embedded == nil {
		return EmbeddedResult{}, launchError("launch_failed", "embedded AI sessions are unavailable")
	}
	s.embeddedLaunchMu.Lock()
	defer s.embeddedLaunchMu.Unlock()
	if result, found, err := s.existingSession(input.WorkspaceID, input.IdempotencyKey); err != nil || found {
		return result, err
	}
	providerID := strings.TrimSpace(input.Provider)
	provider, capability, err := s.embeddedProvider(providerID)
	if err != nil {
		return EmbeddedResult{}, err
	}
	values := map[string]string{"workspace": input.Dir, "contextFile": "run worktree", "itemPath": "run worktree", "identifier": input.Branch, "contextMode": "workspace_only", "intent": ReviewIntent, "prompt": input.Prompt}
	recordID, args := embeddedSessionID(providerID, provider, launchProviderArgs("workspace_only", provider.Args, values))
	now := s.sessionNow()
	record, err := s.persistStartingSession(SessionRecord{ID: recordID, WorkspaceID: input.WorkspaceID, Provider: providerID, Intent: ReviewIntent, RequestedBranch: input.Branch, IdempotencyKey: input.IdempotencyKey, State: StateStarting, StartedAt: now, LastKnownAt: now})
	if err != nil {
		return EmbeddedResult{}, launchErrorWith("launch_failed", err)
	}
	return s.startRecorded(record, StartRequest{ID: record.ID, WorkspaceID: input.WorkspaceID, Provider: providerID, Intent: ReviewIntent, Executable: capability.Executable, Args: args, Dir: input.Dir, Columns: input.Columns, Rows: input.Rows})
}
