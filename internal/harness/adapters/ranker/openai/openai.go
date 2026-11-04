// Package openai is the sorter's optional re-ranker: it asks a model behind
// an OpenAI-compatible chat endpoint (the local Qwen by default) to reorder
// the rules' top N tasks, and applies only a valid permutation of them. Task
// text goes to the model inside task-data delimiters; the model's answer is
// an order of ids, never an instruction.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kode-stream/internal/harness/adapters/credential"
	"kode-stream/internal/harness/application/brief"
	"kode-stream/internal/harness/application/queue"
	"kode-stream/internal/harness/domain"
)

const maxResponseBytes = 1 << 20

type Reranker struct {
	HTTPClient *http.Client
	Tokens     credential.Resolver
}

func New() *Reranker {
	return &Reranker{HTTPClient: &http.Client{Timeout: 60 * time.Second}, Tokens: credential.Default()}
}

const systemPrompt = "You order engineering tasks for an automated coding pipeline. Reply with one JSON object {\"order\": [task ids, most valuable to do first]} listing every id you were given exactly once, and nothing else."

type candidate struct {
	ID       string   `json:"id"`
	Key      string   `json:"key"`
	Title    string   `json:"title"`
	Priority string   `json:"priority,omitempty"`
	Labels   []string `json:"labels,omitempty"`
	Score    float64  `json:"score"`
	Waits    int      `json:"waitsFor,omitempty"`
}

func (r *Reranker) Rerank(ctx context.Context, ranked []queue.Scored, settings domain.Settings) ([]queue.Scored, error) {
	config := settings.Ranker.LLM
	if !config.Enabled {
		return ranked, nil
	}
	topN := min(config.TopN, len(ranked))
	var candidates []candidate
	for _, entry := range ranked[:topN] {
		if entry.Task.Pinned {
			continue
		}
		title := entry.Task.Title
		if len([]rune(title)) > 200 {
			title = string([]rune(title)[:200])
		}
		candidates = append(candidates, candidate{ID: entry.Task.ID, Key: entry.Task.Source.Key, Title: title, Priority: entry.Task.Source.Priority, Labels: entry.Task.Source.Labels, Score: entry.Score, Waits: len(entry.Task.DependsOn)})
	}
	if len(candidates) < 2 {
		return ranked, nil
	}
	listing, _ := json.MarshalIndent(candidates, "", "  ")
	user := "Order these tasks. Prefer unblocking work and higher priority; keep ties in the given order.\n\n" + brief.Data(string(listing))
	order, err := r.ask(ctx, config, user)
	if err != nil {
		return ranked, err
	}
	reordered, ok := queue.ApplyOrder(ranked, topN, order)
	if !ok {
		return ranked, fmt.Errorf("the model's order is not a permutation of the %d tasks it was given", len(candidates))
	}
	return reordered, nil
}

func (r *Reranker) ask(ctx context.Context, config domain.LLMRankerSettings, user string) ([]string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":       config.Model,
		"temperature": 0,
		"messages":    []map[string]string{{"role": "system", "content": systemPrompt}, {"role": "user", "content": user}},
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(config.Endpoint, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if token, err := r.Tokens.Resolve(config.APIKeyEnvVar); err != nil {
		return nil, err
	} else if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := r.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("the ranking model is unavailable: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the ranking model answered %d", response.StatusCode)
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("the ranking model's answer exceeds the size limit")
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &completion); err != nil || len(completion.Choices) == 0 {
		return nil, fmt.Errorf("the ranking model's answer has no choice")
	}
	content := completion.Choices[0].Message.Content
	start, end := strings.Index(content, "{"), strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("the ranking model's answer has no JSON object")
	}
	var answer struct {
		Order []string `json:"order"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &answer); err != nil {
		return nil, fmt.Errorf("the ranking model's answer is not an order: %w", err)
	}
	return answer.Order, nil
}
