package main

import "testing"

func TestRunAgentParsesSupportedCommands(t *testing.T) {
	if err := runAgent([]string{"status"}); err != nil {
		t.Fatalf("status err = %v", err)
	}
	if err := runAgent([]string{"doctor", "--cloud-url", "https://cloud.example.com", "--repo", "/repo"}); err != nil {
		t.Fatalf("doctor err = %v", err)
	}
	if err := runAgent([]string{"start", "--connect", "kodestream://connect?token=one", "--cloud-url", "https://cloud.example.com"}); err != nil {
		t.Fatalf("start err = %v", err)
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
