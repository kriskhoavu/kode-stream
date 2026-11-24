// Package harness moves ranked tasks through plan, build, test and draft pull request by running coding CLIs headless.
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

// Secrets live in Config and the process only, never in the database.
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
	Repository           domain.Repository
	Workspaces           ports.Workspaces
	Scouts               []ports.Scout
	Engines              []ports.Engine
	Worktrees            ports.Worktrees
	PullRequests         []ports.PullRequests
	Notifier             ports.Notifier
	Audit                ports.Audit
	RuntimeVerifyCommand func(*models.WorkspaceRuntimeConfig) string
	Reranker             queue.Reranker
	Transcripts          ports.TranscriptReader
	Sessions             ports.SessionCloser
	Now                  func() time.Time
	Config               Config
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

func wake(channel chan struct{}) {
	select {
	case channel <- struct{}{}:
	default:
	}
}

func (s *Service) wakeWork() {
	for _, channel := range s.workWakes {
		wake(channel)
	}
}

func (s *Service) Supported() bool { return s != nil && s.repository != nil }

func (s *Service) Config() Config { return s.config }

func (s *Service) mutable() error {
	if !s.Supported() {
		return domain.ErrUnsupported
	}
	if !s.config.LocalMode {
		return domain.ErrUnavailable
	}
	return nil
}

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

// The caller publishes the event after the commit.
func emitIn(ctx context.Context, tx domain.Repository, event domain.Event) (domain.Event, error) {
	return tx.AppendEvent(ctx, event)
}

func (s *Service) publish(events ...domain.Event) {
	for _, event := range events {
		s.feed.Publish(event)
	}
}

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

func (s *Service) Subscribe(buffer int) *feed.Subscription { return s.feed.Subscribe(buffer) }

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
