package main

import "testing"

func TestRunAgentParsesSupportedCommands(t *testing.T) {
	if err := runAgent([]string{"status"}); err != nil {
		t.Fatalf("status err = %v", err)
	}
	if err := runAgent([]string{"doctor", "--cloud-url", "https://cloud.example.com", "--repo", "/repo"}); err != nil {
		t.Fatalf("doctor err = %v", err)
	}
}

func TestRunAgentRejectsMissingOrUnsupportedCommand(t *testing.T) {
	if err := runAgent(nil); err == nil {
		t.Fatal("expected missing command error")
	}
	if err := runAgent([]string{"install"}); err == nil {
		t.Fatal("expected unsupported command error")
	}
}

func TestRunAgentStartValidatesRequiredConnectionInput(t *testing.T) {
	if err := runAgent([]string{"start"}); err == nil {
		t.Fatal("expected start validation error")
	}
}
