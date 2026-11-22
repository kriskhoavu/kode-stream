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
	// Notifier delivers the bot's webhooks; nil when no webhook URL is set.
	Notifier ports.Notifier
	Audit    ports.Audit
	// RuntimeVerifyCommand returns a workspace runtime's smoke verification
	// command, or "" when it has none.
	RuntimeVerifyCommand func(*models.WorkspaceRuntimeConfig) string
	// Reranker may reorder the rules' top N when the settings enable it.
	Reranker queue.Reranker
	// Transcripts reads an agent run's transcript for the dashboard; nil
	// when no engine writes one the platform can read.
	Transcripts ports.TranscriptReader
	// Sessions closes a person's sessions in a run's working copy before
	// Remove or Reset deletes it; nil where no session can run there.
	Sessions ports.SessionCloser
	Now      func() time.Time
	Config   Config
}

type Service struct {
	repository  domain.Repository
	workspaces  ports.Workspaces
	now         func() time.Time
	config      Config
	feed        *feed.Broadcaster
	poller      *scout.Poller
	rules       queue.RuleRanker
	reranker    queue.Reranker
	transcripts ports.TranscriptReader
	sessions    ports.SessionCloser

	engines    map[string]ports.Engine
	engineList []ports.Engine
	worktrees  ports.Worktrees
	pulls      []ports.PullRequests
	notifier   ports.Notifier
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
	service := &Service{repository: deps.Repository, workspaces: deps.Workspaces, now: func() time.Time { return now().UTC() }, config: deps.Config, feed: feed.NewBroadcaster(), reranker: deps.Reranker, transcripts: deps.Transcripts, sessions: deps.Sessions, progress: map[string]string{},
		pollWake: make(chan struct{}, 1), sortWake: make(chan struct{}, 1), workWakes: map[string]chan struct{}{},
		engines: map[string]ports.Engine{}, engineList: deps.Engines, worktrees: deps.Worktrees, pulls: deps.PullRequests, notifier: deps.Notifier, audit: deps.Audit, runtimeCmd: deps.RuntimeVerifyCommand, handles: map[string]agentProcess{}, busy: map[domain.RoleKey]int{}}
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
	_ = s.enqueueFor(ctx, s.repository, stored)
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
	if err := s.checkEngines(normalized.ModelPolicy); err != nil {
		return domain.Settings{}, err
	}
	for _, workspace := range normalized.Workspaces {
		if err := s.checkEngines(normalized.For(workspace.WorkspaceID).ModelPolicy); err != nil {
			return domain.Settings{}, domain.Invalid("workspace %s: %v", workspace.WorkspaceID, err)
		}
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

// checkEngines refuses a policy that names an engine this server does not
// run, or an OpenCode model without its provider.
func (s *Service) checkEngines(policy domain.ModelPolicy) error {
	if len(s.engines) == 0 {
		return nil
	}
	for _, role := range domain.Roles {
		rolePolicy := policy.Roles[role]
		if _, ok := s.engines[rolePolicy.Engine]; !ok {
			return domain.Invalid("modelPolicy.roles.%s.engine %q is not available; choose one of the engines in status", role, rolePolicy.Engine)
		}
		for _, model := range []string{rolePolicy.Model, rolePolicy.HighDifficultyModel} {
			if rolePolicy.Engine == "opencode" && model != "" && !strings.Contains(model, "/") {
				return domain.Invalid("modelPolicy.roles.%s: OpenCode models are named provider/model, not %q", role, model)
			}
		}
	}
	return nil
}

func actorOrNil(actor domain.Actor) *domain.Actor {
	if actor.ActorID == "" && actor.DisplayName == "" {
		return nil
	}
	return &actor
}

// --- tasks ---

// --- runs ---

// --- approvals ---

// --- status ---

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
