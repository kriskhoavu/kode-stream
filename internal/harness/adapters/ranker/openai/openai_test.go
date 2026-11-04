package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kode-stream/internal/harness/adapters/credential"
	"kode-stream/internal/harness/application/brief"
	"kode-stream/internal/harness/application/queue"
	"kode-stream/internal/harness/domain"
)

func TestRerankerAppliesAValidOrderAndRefusesAnInvalidOne(t *testing.T) {
	answer := `Sure. {"order":["c","a","b"]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/v1/chat/completions" || body.Model != "qwen" || r.Header.Get("Authorization") != "Bearer k" || !strings.Contains(body.Messages[1].Content, brief.DataNotice) || strings.Contains(body.Messages[1].Content, "\"pin\"") {
			t.Fatalf("request = %s %#v", r.URL.Path, body)
		}
		data, _ := json.Marshal(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": answer}}}})
		_, _ = w.Write(data)
	}))
	defer server.Close()
	settings := domain.DefaultSettings()
	settings.Ranker.LLM = domain.LLMRankerSettings{Enabled: true, Endpoint: server.URL + "/v1", Model: "qwen", APIKeyEnvVar: "QWEN_KEY", TopN: 4}
	ranked := []queue.Scored{
		{Task: domain.Task{ID: "pin", Pinned: true}, Score: 1000},
		{Task: domain.Task{ID: "a", Title: "A"}, Score: 40},
		{Task: domain.Task{ID: "b", Title: "B"}, Score: 30},
		{Task: domain.Task{ID: "c", Title: "C"}, Score: 20},
		{Task: domain.Task{ID: "d", Title: "D"}, Score: 10},
	}
	reranker := &Reranker{HTTPClient: server.Client(), Tokens: credential.Resolver{Getenv: func(string) string { return "k" }}}
	out, err := reranker.Rerank(context.Background(), ranked, settings)
	if err != nil || out[0].Task.ID != "pin" || out[1].Task.ID != "c" || out[1].Score != 40 || out[4].Task.ID != "d" {
		t.Fatalf("out = %#v err=%v", out, err)
	}
	answer = `{"order":["a","d","b"]}`
	out, err = reranker.Rerank(context.Background(), ranked, settings)
	if err == nil || out[1].Task.ID != "a" {
		t.Fatalf("an order naming a task outside the top N was applied: err=%v", err)
	}
	settings.Ranker.LLM.Enabled = false
	if out, err := reranker.Rerank(context.Background(), ranked, settings); err != nil || out[1].Task.ID != "a" {
		t.Fatal("a disabled re-ranker changed the order")
	}
}
