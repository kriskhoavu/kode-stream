package e2eresult

import (
	"strings"
	"testing"
)

func TestParseRequiresVersionedMatchingBoundedEvidence(t *testing.T) {
	runbook := []byte("# Scenario\n")
	valid := `{"version":1,"status":"passed","runbookFingerprint":"` + Fingerprint(runbook) + `","recordedAt":"2026-08-13T00:00:00Z","evidence":["plans/a/artifacts/result.png"]}`
	result, diagnostic := Parse([]byte(valid), runbook)
	if result.Status != "passed" || result.Freshness != "fresh" || diagnostic != "" {
		t.Fatalf("result=%#v diagnostic=%q", result, diagnostic)
	}
	stale, _ := Parse([]byte(valid), []byte("# Changed\n"))
	if stale.Freshness != "stale" {
		t.Fatalf("stale=%#v", stale)
	}
	for _, content := range [][]byte{[]byte("Status: passed"), []byte(`{"version":1,"status":"passed"}`), []byte(strings.Repeat("x", MaxBytes+1))} {
		result, _ := Parse(content, runbook)
		if result.Freshness != "unknown" {
			t.Fatalf("unsafe result accepted: %#v", result)
		}
	}
}

func TestParseRejectsUnsafeEvidence(t *testing.T) {
	runbook := []byte("# Scenario\n")
	content := `{"version":1,"status":"passed","runbookFingerprint":"` + Fingerprint(runbook) + `","recordedAt":"2026-08-13T00:00:00Z","evidence":["../secret"]}`
	result, _ := Parse([]byte(content), runbook)
	if result.Freshness != "unknown" {
		t.Fatalf("result=%#v", result)
	}
}
