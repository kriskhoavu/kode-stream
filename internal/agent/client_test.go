package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	appgit "kode-stream/internal/git"
)

func TestClientConnectsSendsHeartbeatAndPublishesWorkspaceMetadata(t *testing.T) {
	repo := newAgentGitRepo(t)
	var published WorkspaceMetadata
	receivedHeartbeat := make(chan struct{})
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agents/channel", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "token" {
			http.Error(w, "bad token", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if err := conn.WriteJSON(Frame{Type: FrameConnected}); err != nil {
			return
		}
		for {
			var frame Frame
			if err := conn.ReadJSON(&frame); err != nil {
				return
			}
			if frame.Type == FrameHeartbeat {
				_ = conn.WriteJSON(Frame{Type: FrameHeartbeatAck})
				close(receivedHeartbeat)
				return
			}
		}
	})
	mux.HandleFunc("/api/workspaces/from-agent", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			http.Error(w, "bad auth", http.StatusUnauthorized)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&published); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := NewClient(Config{
		CloudURL:          server.URL,
		Connect:           "kodestream://connect?token=token",
		Repo:              repo,
		Git:               appgit.New(),
		HeartbeatInterval: 10 * time.Millisecond,
		OnFrame: func(frame Frame) {
			if frame.Type == FrameHeartbeatAck {
				cancel()
			}
		},
	})
	err := client.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("run err = %v", err)
	}
	select {
	case <-receivedHeartbeat:
	default:
		t.Fatal("server did not receive heartbeat")
	}
	if published.Name != filepath.Base(repo) || published.BaselineBranch != "main" || len(published.Sources) != 1 || published.Sources[0] != "plans" || !published.PublishedSummary {
		t.Fatalf("published = %#v", published)
	}
}

func TestBuildWorkspaceMetadataRejectsNonGitRepo(t *testing.T) {
	if _, err := BuildWorkspaceMetadata(t.TempDir(), appgit.New()); err == nil {
		t.Fatal("expected non-git repo error")
	}
}

func newAgentGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "README.md"), []byte("# Plans\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "-c", "user.name=Agent", "-c", "user.email=agent@example.com", "commit", "-m", "seed")
	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
