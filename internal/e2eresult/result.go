// Package e2eresult defines the bounded evidence format shared by plan and
// knowledge E2E surfaces. Legacy Markdown is deliberately readable only as
// unknown evidence: it has no proof that it belongs to the current runbook.
package e2eresult

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kode-stream/internal/common/models"
)

const MaxBytes = 128 << 10
const Version = 1
const MaxRunbookBytes = 1 << 20

type document struct {
	Version            int       `json:"version"`
	Status             string    `json:"status"`
	RunbookFingerprint string    `json:"runbookFingerprint"`
	RecordedAt         time.Time `json:"recordedAt"`
	Provider           string    `json:"provider,omitempty"`
	Environment        string    `json:"environment,omitempty"`
	FailedStep         string    `json:"failedStep,omitempty"`
	Evidence           []string  `json:"evidence"`
}

func Fingerprint(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// ReadFile bounds an untrusted agent-written latest-result before it reaches a
// decoder. It deliberately returns the oversized sentinel data to Parse so the
// caller receives the stable unknown-result diagnostic rather than an I/O leak.
func ReadFile(path string) ([]byte, error) {
	return ReadFileLimit(path, MaxBytes)
}

func ReadFileLimit(path string, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, errors.New("read limit must be positive")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, limit+1))
}

func Parse(data, runbook []byte) (*models.E2ELatestResult, string) {
	unknown := func(reason string) (*models.E2ELatestResult, string) {
		return &models.E2ELatestResult{Status: "unknown", Freshness: "unknown", Evidence: []string{}}, reason
	}
	if len(data) > MaxBytes {
		return unknown("Latest E2E result exceeds the supported size.")
	}
	var value document
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return unknown("Latest E2E result uses a legacy or invalid format.")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return unknown("Latest E2E result is ambiguous.")
	}
	if value.Version != Version || value.RecordedAt.IsZero() || !validStatus(value.Status) || value.RunbookFingerprint == "" {
		return unknown("Latest E2E result is incomplete or unsupported.")
	}
	if value.RunbookFingerprint != Fingerprint(runbook) {
		return &models.E2ELatestResult{Status: value.Status, Freshness: "stale", Evidence: []string{}}, "Latest E2E result is stale for this runbook."
	}
	if len(value.Evidence) > 32 {
		return unknown("Latest E2E result contains too many evidence paths.")
	}
	evidence := make([]string, 0, len(value.Evidence))
	for _, path := range value.Evidence {
		path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if path == "" || path == "." || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, "../") {
			return unknown("Latest E2E result contains an unsafe evidence path.")
		}
		evidence = append(evidence, path)
	}
	return &models.E2ELatestResult{Status: value.Status, Freshness: "fresh", Provider: value.Provider, Environment: value.Environment, FailedStep: value.FailedStep, Evidence: evidence, RecordedAt: value.RecordedAt}, ""
}

func validStatus(status string) bool {
	switch status {
	case "passed", "failed", "blocked":
		return true
	}
	return false
}

var ErrInvalid = errors.New("invalid E2E result")
