// Package harness is the engineering harness: scouts fill a ranked queue of
// tasks, and an orchestrator moves each task through plan, approval, build,
// test and draft pull request stages by running coding CLIs headless.
//
// Only the composition root, the HTTP transport and the SQL repository
// import this tree; deleting it and those references removes the feature.
package harness

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"kode-stream/internal/common/models"
	"kode-stream/internal/harness/application/feed"
	"kode-stream/internal/harness/application/queue"
	"kode-stream/internal/harness/application/scheduler"
	"kode-stream/internal/harness/application/scout"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

const (
	EnvBotToken      = "KODE_STREAM_HARNESS_BOT_TOKEN"
	EnvWebhookURL    = "KODE_STREAM_HARNESS_BOT_WEBHOOK_URL"
	EnvWebhookSecret = "KODE_STREAM_HARNESS_BOT_WEBHOOK_SECRET"
	EnvWorktreeRoot  = "KODE_STREAM_HARNESS_WORKTREE_ROOT"
	EnvDashboardURL  = "KODE_STREAM_HARNESS_DASHBOARD_URL"
)

// Config is resolved once from the environment by the composition root.
// Secrets live here and in the process only, never in the database.
type Config struct {
	LocalMode     bool
	BotToken      string
	WebhookURL    string
	WebhookSecret string
	WorktreeRoot  string
	DashboardURL  string
}

func ResolveConfig(getenv func(string) string, dataDir string, localMode bool) Config {
	root := strings.TrimSpace(getenv(EnvWorktreeRoot))
	if root == "" && dataDir != "" {
		root = filepath.Join(dataDir, "harness", "worktrees")
	}
	dashboard := strings.TrimRight(strings.TrimSpace(getenv(EnvDashboardURL)), "/")
	return Config{
		LocalMode:     localMode,
		BotToken:      strings.TrimSpace(getenv(EnvBotToken)),
		WebhookURL:    strings.TrimSpace(getenv(EnvWebhookURL)),
		WebhookSecret: getenv(EnvWebhookSecret),
		WorktreeRoot:  root,
		DashboardURL:  dashboard,
	}
}

type Dependencies struct {
	// Repository is nil when the storage option cannot hold harness state.
	Repository   domain.Repository
	Workspaces   ports.Workspaces
	Scouts       []ports.Scout
	Engines      []ports.Engine
	Worktrees    ports.Worktrees
	PullRequests []ports.PullRequests
	Audit        ports.Audit
	// RuntimeVerifyCommand returns a workspace runtime's smoke verification
	// command, or "" when it has none.
	RuntimeVerifyCommand func(*models.WorkspaceRuntimeConfig) string
	// Ranker re-orders the rules' ranking; nil keeps the rules' order.
	Ranker queue.Ranker
	Now    func() time.Time
	Config Config
}

type Service struct {
	repository domain.Repository
	workspaces ports.Workspaces
	now        func() time.Time
	config     Config
	feed       *feed.Broadcaster
	poller     *scout.Poller
	rules      queue.RuleRanker
	ranker     queue.Ranker

	engines    map[string]ports.Engine
	engineList []ports.Engine
	worktrees  ports.Worktrees
	pulls      []ports.PullRequests
	audit      ports.Audit
	runtimeCmd func(*models.WorkspaceRuntimeConfig) string

	pollWake  chan struct{}
	sortWake  chan struct{}
	workWakes map[string]chan struct{}

	mu             sync.Mutex
	running        bool
	orchestrator   *scheduler.Orchestrator
	progress       map[string]string
	handles        map[string]agentProcess
	busy           map[domain.RoleKey]int
	workCtx        context.Context
	workCancel     context.CancelFunc
	detections     []EngineStatus
	detectedAt     time.Time
	dispatchCursor int
}

func New(deps Dependencies) *Service {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	service := &Service{repository: deps.Repository, workspaces: deps.Workspaces, now: func() time.Time { return now().UTC() }, config: deps.Config, feed: feed.NewBroadcaster(), ranker: deps.Ranker, progress: map[string]string{},
		pollWake: make(chan struct{}, 1), sortWake: make(chan struct{}, 1), workWakes: map[string]chan struct{}{},
		engines: map[string]ports.Engine{}, engineList: deps.Engines, worktrees: deps.Worktrees, pulls: deps.PullRequests, audit: deps.Audit, runtimeCmd: deps.RuntimeVerifyCommand, handles: map[string]agentProcess{}, busy: map[domain.RoleKey]int{}}
	for _, name := range workLoopNames {
		service.workWakes[name] = make(chan struct{}, 1)
	}
	for _, engine := range deps.Engines {
		service.engines[engine.ID()] = engine
	}
	if deps.Repository != nil && deps.Workspaces != nil {
		service.poller = scout.NewPoller(deps.Repository, deps.Workspaces, deps.Scouts, service.now, service.emit)
	}
	return service
}

// wake nudges a loop without blocking; a wake already pending is enough.
func wake(channel chan struct{}) {
	select {
	case channel <- struct{}{}:
	default:
	}
}

// wakeWork nudges the dispatcher, every role loop and the stage loop.
func (s *Service) wakeWork() {
	for _, channel := range s.workWakes {
		wake(channel)
	}
}

// Supported reports whether the storage option holds harness state.
func (s *Service) Supported() bool { return s != nil && s.repository != nil }

// Config returns the resolved configuration; the transport reads the bot
// token from it.
func (s *Service) Config() Config { return s.config }

// mutable refuses a change the harness cannot make in this process.
func (s *Service) mutable() error {
	if !s.Supported() {
		return domain.ErrUnsupported
	}
	if !s.config.LocalMode {
		return domain.ErrUnavailable
	}
	return nil
}

// emit stores an event and publishes it with its sequence number.
func (s *Service) emit(ctx context.Context, event domain.Event) domain.Event {
	if event.At.IsZero() {
		event.At = s.now()
	}
	stored, err := s.repository.AppendEvent(ctx, event)
	if err != nil {
		return event
	}
	s.feed.Publish(stored)
	return stored
}

// emitIn stores an event inside a transaction; the caller publishes it after
// the commit with publish.
func emitIn(ctx context.Context, tx domain.Repository, event domain.Event) (domain.Event, error) {
	return tx.AppendEvent(ctx, event)
}

func (s *Service) publish(events ...domain.Event) {
	for _, event := range events {
		s.feed.Publish(event)
	}
}

// publishTransient sends an event that is not stored, such as live progress.
func (s *Service) publishTransient(event domain.Event) {
	if event.At.IsZero() {
		event.At = s.now()
	}
	s.feed.Publish(event)
}

func payload(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil || len(data) > domain.MaxPayloadBytes {
		return json.RawMessage(`{}`)
	}
	return data
}

// --- settings ---

func (s *Service) Settings(ctx context.Context) (domain.Settings, error) {
	if !s.Supported() {
		return domain.DefaultSettings(), nil
	}
	return s.repository.LoadSettings(ctx)
}

func (s *Service) SaveSettings(ctx context.Context, settings domain.Settings, expectedRevision int64, actor domain.Actor) (domain.Settings, error) {
	if err := s.mutable(); err != nil {
		return domain.Settings{}, err
	}
	normalized, err := settings.Normalize()
	if err != nil {
		return domain.Settings{}, err
	}
	if s.workspaces != nil {
		for _, workspace := range normalized.Workspaces {
			if _, found, err := s.workspaces.Get(workspace.WorkspaceID); err != nil {
				return domain.Settings{}, err
			} else if !found {
				return domain.Settings{}, domain.Invalid("workspace %s is not registered", workspace.WorkspaceID)
			}
		}
	}
	saved, err := s.repository.SaveSettings(ctx, normalized, expectedRevision)
	if err != nil {
		return domain.Settings{}, err
	}
	s.emit(ctx, domain.Event{Kind: domain.EventSettingsUpdated, Actor: actorOrNil(actor), Payload: payload(map[string]any{"revision": saved.Revision, "enabled": saved.Enabled})})
	return saved, nil
}

func actorOrNil(actor domain.Actor) *domain.Actor {
	if actor.ActorID == "" && actor.DisplayName == "" {
		return nil
	}
	return &actor
}

// --- tasks ---

func (s *Service) Tasks(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, error) {
	if !s.Supported() {
		return []domain.Task{}, nil
	}
	tasks, err := s.repository.ListTasks(ctx, filter)
	if err != nil {
		return nil, err
	}
	return withoutBodies(tasks), nil
}

func withoutBodies(tasks []domain.Task) []domain.Task {
	for i := range tasks {
		tasks[i].Body = ""
	}
	return tasks
}

func (s *Service) Task(ctx context.Context, id string) (domain.Task, error) {
	if !s.Supported() {
		return domain.Task{}, domain.ErrNotFound
	}
	return s.repository.GetTask(ctx, id)
}

// Queue lists the tasks that can still be worked, in the order the
// dispatcher would take them: pinned first, then by score.
func (s *Service) Queue(ctx context.Context, workspaceID string, limit int) ([]domain.Task, error) {
	return s.Tasks(ctx, domain.TaskFilter{WorkspaceID: workspaceID, States: []domain.TaskState{domain.TaskRanked, domain.TaskDiscovered, domain.TaskRunning, domain.TaskBlocked}, Limit: limit})
}

// --- runs ---

type StageView struct {
	domain.Stage
	AgentRuns []domain.AgentRunSummary `json:"agentRuns"`
}

type RunView struct {
	domain.Run
	Stages []StageView `json:"stages,omitempty"`
}

func (s *Service) Runs(ctx context.Context, filter domain.RunFilter) ([]domain.Run, error) {
	if !s.Supported() {
		return []domain.Run{}, nil
	}
	return s.repository.ListRuns(ctx, filter)
}

func (s *Service) Run(ctx context.Context, id string) (RunView, error) {
	if !s.Supported() {
		return RunView{}, domain.ErrNotFound
	}
	run, err := s.repository.GetRun(ctx, id)
	if err != nil {
		return RunView{}, err
	}
	stages, err := s.repository.ListStages(ctx, id)
	if err != nil {
		return RunView{}, err
	}
	agentRuns, err := s.repository.ListAgentRuns(ctx, domain.AgentRunFilter{RunID: id, Limit: 1000})
	if err != nil {
		return RunView{}, err
	}
	byStage := map[string][]domain.AgentRunSummary{}
	for _, agentRun := range agentRuns {
		summary := agentRun.Summary()
		summary.Progress = s.progressOf(agentRun.ID)
		byStage[agentRun.StageID] = append(byStage[agentRun.StageID], summary)
	}
	view := RunView{Run: run, Stages: make([]StageView, 0, len(stages))}
	for _, stage := range stages {
		runs := byStage[stage.ID]
		if runs == nil {
			runs = []domain.AgentRunSummary{}
		}
		view.Stages = append(view.Stages, StageView{Stage: stage, AgentRuns: runs})
	}
	return view, nil
}

func (s *Service) AgentRun(ctx context.Context, id string) (domain.AgentRun, error) {
	if !s.Supported() {
		return domain.AgentRun{}, domain.ErrNotFound
	}
	agentRun, err := s.repository.GetAgentRun(ctx, id)
	if err != nil {
		return agentRun, err
	}
	if agentRun.State == domain.AgentRunning {
		agentRun.Progress = s.progressOf(id)
	}
	return agentRun, nil
}

func (s *Service) progressOf(agentRunID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress[agentRunID]
}

func (s *Service) setProgress(agentRunID, progress string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if progress == "" {
		delete(s.progress, agentRunID)
		return
	}
	s.progress[agentRunID] = progress
}

// --- approvals ---

func (s *Service) Approvals(ctx context.Context, filter domain.ApprovalFilter) ([]domain.Approval, error) {
	if !s.Supported() {
		return []domain.Approval{}, nil
	}
	return s.repository.ListApprovals(ctx, filter)
}

// --- status ---

type EngineStatus struct {
	ID          string   `json:"id"`
	Available   bool     `json:"available"`
	Version     string   `json:"version,omitempty"`
	Path        string   `json:"path,omitempty"`
	Unsupported []string `json:"unsupported"`
}

type RoleHealth struct {
	Role           domain.RoleKey `json:"role"`
	Paused         bool           `json:"paused"`
	Slots          int            `json:"slots"`
	Busy           int            `json:"busy"`
	LastActivityAt *time.Time     `json:"lastActivityAt,omitempty"`
	LastError      string         `json:"lastError,omitempty"`
}

type SourceHealth struct {
	WorkspaceID  string     `json:"workspaceId"`
	Kind         string     `json:"kind"`
	LastPolledAt *time.Time `json:"lastPolledAt,omitempty"`
	Found        int        `json:"found"`
	Added        int        `json:"added"`
	LastError    string     `json:"lastError,omitempty"`
}

type StatusCounts struct {
	WaitingApprovals int `json:"waitingApprovals"`
	Running          int `json:"running"`
	Queued           int `json:"queued"`
	DoneToday        int `json:"doneToday"`
	Blocked          int `json:"blocked"`
}

type Status struct {
	Enabled          bool           `json:"enabled"`
	StorageSupported bool           `json:"storageSupported"`
	Mode             string         `json:"mode"`
	Running          bool           `json:"running"`
	AutoStart        bool           `json:"autoStart"`
	Engines          []EngineStatus `json:"engines"`
	Roles            []RoleHealth   `json:"roles"`
	Sources          []SourceHealth `json:"sources"`
	Counts           StatusCounts   `json:"counts"`
	CostTodayUSD     float64        `json:"costTodayUsd"`
	LastEventSeq     int64          `json:"lastEventSeq"`
	Hint             string         `json:"hint,omitempty"`
}

// StorageHint is the recovery hint for unsupported storage.
const StorageHint = "Switch Storage to SQLite in Settings and restart Kode Stream."

// CloudHint is the recovery hint for Cloud mode.
const CloudHint = "The harness runs coding CLIs on this machine, so it is available in Local mode only."

func (s *Service) Status(ctx context.Context) (Status, error) {
	status := Status{Mode: "local", Engines: []EngineStatus{}, Roles: []RoleHealth{}, Sources: []SourceHealth{}}
	if !s.config.LocalMode {
		status.Mode = "cloud"
		status.Hint = CloudHint
	}
	if !s.Supported() {
		status.Hint = StorageHint
		return status, nil
	}
	status.StorageSupported = true
	settings, err := s.repository.LoadSettings(ctx)
	if err != nil {
		return status, err
	}
	status.Enabled = settings.Enabled
	status.AutoStart = settings.AutoStart
	s.mu.Lock()
	status.Running = s.running
	s.mu.Unlock()
	busy := map[domain.RoleKey]int{}
	running, err := s.repository.ListAgentRuns(ctx, domain.AgentRunFilter{States: []domain.AgentRunState{domain.AgentRunning}, Limit: 1000})
	if err != nil {
		return status, err
	}
	for _, agentRun := range running {
		busy[domain.PoolOf(agentRun.Role)]++
	}
	for _, key := range domain.RoleKeys {
		health := RoleHealth{Role: key, Paused: settings.Paused[key], Slots: settings.Slots.For(key), Busy: busy[key]}
		if key == domain.RoleKeyScout || key == domain.RoleKeySorter {
			health.Slots = 1
		}
		status.Roles = append(status.Roles, s.decorateRole(health))
	}
	status.Sources = s.sourceHealth()
	status.Engines = s.engineStatus(ctx)
	waiting, err := s.repository.ListApprovals(ctx, domain.ApprovalFilter{States: []domain.ApprovalState{domain.ApprovalRequested}, Limit: 1000})
	if err != nil {
		return status, err
	}
	status.Counts.WaitingApprovals = len(waiting)
	tasks, err := s.repository.ListTasks(ctx, domain.TaskFilter{States: []domain.TaskState{domain.TaskRanked, domain.TaskDiscovered, domain.TaskBlocked}, Limit: 1000})
	if err != nil {
		return status, err
	}
	for _, task := range tasks {
		if task.State == domain.TaskBlocked {
			status.Counts.Blocked++
		} else {
			status.Counts.Queued++
		}
	}
	runs, err := s.repository.ListRuns(ctx, domain.RunFilter{States: []domain.RunState{domain.RunRunning}, Limit: 1000})
	if err != nil {
		return status, err
	}
	status.Counts.Running = len(runs)
	dayStart := startOfDay(s.now())
	passed, err := s.repository.ListRuns(ctx, domain.RunFilter{States: []domain.RunState{domain.RunPassed}, Limit: 1000})
	if err != nil {
		return status, err
	}
	for _, run := range passed {
		if run.FinishedAt != nil && !run.FinishedAt.Before(dayStart) {
			status.Counts.DoneToday++
		}
	}
	cost, err := s.repository.CostSummary(ctx, domain.CostQuery{GroupBy: domain.CostByDay, From: dayStart, To: dayStart.Add(24 * time.Hour)})
	if err != nil {
		return status, err
	}
	status.CostTodayUSD = cost.TotalUSD
	status.LastEventSeq, err = s.repository.LastEventSeq(ctx)
	if !settings.Enabled && status.Hint == "" {
		status.Hint = "The harness is off. Turn it on in Harness settings."
	}
	return status, err
}

func startOfDay(now time.Time) time.Time {
	year, month, day := now.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// decorateRole and sourceHealth read the orchestrator's in-memory health;
// engineStatus reads the engines' detection. Each is filled by a later part
// of the service and returns empty data until then.
func (s *Service) decorateRole(health RoleHealth) RoleHealth {
	s.mu.Lock()
	orchestrator := s.orchestrator
	s.mu.Unlock()
	if orchestrator == nil {
		return health
	}
	if loop, ok := orchestrator.Health(string(health.Role)); ok {
		health.LastActivityAt = loop.LastRunAt
		health.LastError = loop.LastError
	}
	return health
}

func (s *Service) sourceHealth() []SourceHealth {
	out := []SourceHealth{}
	if s.poller == nil {
		return out
	}
	for _, health := range s.poller.Health() {
		out = append(out, SourceHealth{WorkspaceID: health.WorkspaceID, Kind: string(health.Kind), LastPolledAt: health.LastPolledAt, Found: health.Found, Added: health.Added, LastError: health.LastError})
	}
	return out
}

// engineStatus detects each engine at most once a minute: detection runs the
// CLI's --version.
func (s *Service) engineStatus(ctx context.Context) []EngineStatus {
	s.mu.Lock()
	if s.detections != nil && s.now().Sub(s.detectedAt) < time.Minute {
		cached := append([]EngineStatus{}, s.detections...)
		s.mu.Unlock()
		return cached
	}
	s.mu.Unlock()
	out := []EngineStatus{}
	for _, engine := range s.engineList {
		detection := engine.Detect(ctx)
		unsupported := detection.Unsupported
		if unsupported == nil {
			unsupported = []string{}
		}
		out = append(out, EngineStatus{ID: engine.ID(), Available: detection.Available, Version: detection.Version, Path: detection.Path, Unsupported: unsupported})
	}
	s.mu.Lock()
	s.detections = out
	s.detectedAt = s.now()
	s.mu.Unlock()
	return out
}

// --- events ---

// EventsAfter returns stored events after seq, at most limit.
func (s *Service) EventsAfter(ctx context.Context, after int64, limit int) ([]domain.Event, error) {
	if !s.Supported() {
		return []domain.Event{}, nil
	}
	return s.repository.EventsAfter(ctx, after, limit)
}

func (s *Service) LastEventSeq(ctx context.Context) (int64, error) {
	if !s.Supported() {
		return 0, nil
	}
	return s.repository.LastEventSeq(ctx)
}

// Subscribe returns a live subscription to every event published from now.
func (s *Service) Subscribe(buffer int) *feed.Subscription { return s.feed.Subscribe(buffer) }

// --- helpers shared by the service files ---

func sortTasksForQueue(tasks []domain.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Pinned != tasks[j].Pinned {
			return tasks[i].Pinned
		}
		if tasks[i].Score != tasks[j].Score {
			return tasks[i].Score > tasks[j].Score
		}
		return tasks[i].DiscoveredAt.Before(tasks[j].DiscoveredAt)
	})
}

// IsNotFound and friends let the transport map errors without importing the
// domain package's sentinels one by one.
func IsNotFound(err error) bool       { return errors.Is(err, domain.ErrNotFound) }
func IsConflict(err error) bool       { return errors.Is(err, domain.ErrStateMoved) }
func IsAlreadyDecided(err error) bool { return errors.Is(err, domain.ErrAlreadyDecided) }
func IsUnsupported(err error) bool    { return errors.Is(err, domain.ErrUnsupported) }
func IsUnavailable(err error) bool    { return errors.Is(err, domain.ErrUnavailable) }
func IsInvalid(err error) bool {
	var invalid domain.InvalidError
	var illegal domain.IllegalTransitionError
	return errors.As(err, &invalid) || errors.As(err, &illegal)
}
