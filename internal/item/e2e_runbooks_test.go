package item

import "testing"

func TestParseE2ELatestResult(t *testing.T) {
	result := parseE2ELatestResult("# E2E Result\n\nStatus: failed\nProvider: playwright\nEnvironment: staging\nFailed step: Open offer\nEvidence: automation/artifacts/failure.png\n")
	if result.Status != "failed" || result.Provider != "playwright" || result.Environment != "staging" || result.FailedStep != "Open offer" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.Evidence) != 1 || result.Evidence[0] != "automation/artifacts/failure.png" {
		t.Fatalf("unexpected evidence: %#v", result.Evidence)
	}
}

func TestParseE2ELatestResultDefaultsToNotRun(t *testing.T) {
	if result := parseE2ELatestResult("# Placeholder\n"); result.Status != "not run" {
		t.Fatalf("status = %q, want not run", result.Status)
	}
}
