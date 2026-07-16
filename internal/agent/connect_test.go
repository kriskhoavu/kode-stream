package agent

import (
	"strings"
	"testing"
)

func TestParseConnectTokenAcceptsRawTokenAndDeepLink(t *testing.T) {
	raw, err := ParseConnectToken(" token.value ")
	if err != nil || raw != "token.value" {
		t.Fatalf("raw token = %q err=%v", raw, err)
	}
	linked, err := ParseConnectToken("kodestream://connect?token=abc.def")
	if err != nil || linked != "abc.def" {
		t.Fatalf("deep link token = %q err=%v", linked, err)
	}
}

func TestParseConnectTokenRejectsInvalidLinks(t *testing.T) {
	for _, value := range []string{"", "kodestream://connect", "kodestream://open?token=one", "https://example.com?token=one"} {
		if _, err := ParseConnectToken(value); err == nil {
			t.Fatalf("expected error for %q", value)
		}
	}
}

func TestChannelURLConvertsHTTPToWebSocket(t *testing.T) {
	got, err := ChannelURL("https://kode-stream.example.com/root", "a b")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "wss://kode-stream.example.com/root/api/agents/channel?") || !strings.Contains(got, "token=a+b") {
		t.Fatalf("channel URL = %q", got)
	}

	got, err = ChannelURL("http://localhost:4318", "token")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ws://localhost:4318/api/agents/channel?token=token" {
		t.Fatalf("channel URL = %q", got)
	}
}
