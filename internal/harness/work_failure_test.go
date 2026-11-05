package harness

import (
	"strings"
	"testing"

	"kode-stream/internal/harness/ports"
)

func TestFailureTextNamesTheCLIsLastWordsWhenItEndedWithoutAStopReason(t *testing.T) {
	loggedOut := failureText(ports.Result{IsError: true, StopReason: "success", ExitCode: 1, FinalText: "Not logged in · Please run /login"})
	if loggedOut != "stopped: Not logged in · Please run /login" {
		t.Fatalf("logged out = %q", loggedOut)
	}
	silent := failureText(ports.Result{IsError: true, ExitCode: 2})
	if silent != "exited with code 2" {
		t.Fatalf("silent = %q", silent)
	}
	budget := failureText(ports.Result{IsError: true, StopReason: "error_max_budget_usd", Stderr: "spent"})
	if !strings.HasPrefix(budget, "stopped at its spending limit: spent") {
		t.Fatalf("budget = %q", budget)
	}
}
