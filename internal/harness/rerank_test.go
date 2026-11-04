package harness

import (
	"context"
	"testing"
	"time"

	"kode-stream/internal/harness/application/queue"
	"kode-stream/internal/harness/domain"
	"kode-stream/internal/harness/ports"
)

type reverseReranker struct{ calls int }

func (r *reverseReranker) Rerank(_ context.Context, ranked []queue.Scored, settings domain.Settings) ([]queue.Scored, error) {
	r.calls++
	order := []string{}
	for i := min(settings.Ranker.LLM.TopN, len(ranked)) - 1; i >= 0; i-- {
		order = append(order, ranked[i].Task.ID)
	}
	out, _ := queue.ApplyOrder(ranked, settings.Ranker.LLM.TopN, order)
	return out, nil
}

func TestTheSorterAppliesTheRerankerOnlyWhenEnabled(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t)
	reranker := &reverseReranker{}
	service := New(Dependencies{Repository: repository, Workspaces: fakeWorkspaces{"ws": {ID: "ws"}}, Scouts: []ports.Scout{&staticScout{candidates: []domain.Candidate{jiraCandidate("PM-1", "Highest"), jiraCandidate("PM-2", "Low")}}}, Reranker: reranker, Now: func() time.Time { return testNow }, Config: Config{LocalMode: true}})
	settings := enabledSettings(t, service)
	if _, err := service.PollNow(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if queue, _ := service.Queue(ctx, "", 10); queue[0].Source.Key != "PM-1" || reranker.calls != 0 {
		t.Fatalf("rules order changed while the re-ranker is off: %s calls=%d", queue[0].Source.Key, reranker.calls)
	}
	settings.Ranker.LLM = domain.LLMRankerSettings{Enabled: true, Endpoint: "http://127.0.0.1:1/v1", Model: "qwen", TopN: 2}
	if _, err := service.SaveSettings(ctx, settings, settings.Revision, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	if err := service.Sort(ctx); err != nil {
		t.Fatal(err)
	}
	queue, _ := service.Queue(ctx, "", 10)
	if reranker.calls != 1 || queue[0].Source.Key != "PM-2" || queue[0].ScoreReasons[len(queue[0].ScoreReasons)-1] != "reordered by the model" {
		t.Fatalf("queue = %s %v calls=%d", queue[0].Source.Key, queue[0].ScoreReasons, reranker.calls)
	}
}

func TestSettingsRefuseEnginesTheServerDoesNotRun(t *testing.T) {
	ctx := context.Background()
	repository := newTestRepository(t)
	service := New(Dependencies{Repository: repository, Workspaces: fakeWorkspaces{"ws": {ID: "ws"}}, Engines: []ports.Engine{newFakeEngine(happyScript), namedEngine{"opencode"}}, Config: Config{LocalMode: true}})
	settings := domain.DefaultSettings()
	settings.ModelPolicy.Roles[domain.RoleCoder] = domain.RolePolicy{Engine: "codex", Model: "gpt-5"}
	if _, err := service.SaveSettings(ctx, settings, 0, domain.Actor{}); !IsInvalid(err) {
		t.Fatalf("unknown engine err = %v", err)
	}
	settings.ModelPolicy.Roles[domain.RoleCoder] = domain.RolePolicy{Engine: "opencode", Model: "sonnet"}
	if _, err := service.SaveSettings(ctx, settings, 0, domain.Actor{}); !IsInvalid(err) {
		t.Fatalf("opencode model err = %v", err)
	}
	settings.ModelPolicy.Roles[domain.RoleCoder] = domain.RolePolicy{Engine: "opencode", Model: "anthropic/claude-sonnet"}
	if _, err := service.SaveSettings(ctx, settings, 0, domain.Actor{}); err != nil {
		t.Fatal(err)
	}
	status, _ := service.Status(ctx)
	if len(status.Engines) != 2 {
		t.Fatalf("engines = %#v", status.Engines)
	}
}

type namedEngine struct{ id string }

func (n namedEngine) ID() string { return n.id }
func (n namedEngine) Detect(context.Context) ports.Detection {
	return ports.Detection{Unsupported: []string{"maxTurns"}}
}
func (n namedEngine) Start(context.Context, ports.Invocation) (ports.Handle, error) { return nil, nil }
