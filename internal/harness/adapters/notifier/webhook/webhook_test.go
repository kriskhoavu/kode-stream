package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"kode-stream/internal/harness/ports"
)

func TestDeliverSignsTheBodyAndChecksTheStatus(t *testing.T) {
	status := http.StatusAccepted
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("X-Harness-Signature") != Signature("secret", body) || r.Header.Get("X-Harness-Event-Id") != "evt-7" || r.Header.Get("X-Harness-Attempt") != "2" {
			t.Fatalf("headers = %v", r.Header)
		}
		w.WriteHeader(status)
	}))
	defer server.Close()
	notifier := New(server.URL, "secret")
	delivery := ports.Delivery{EventID: "evt-7", Attempt: 2, Body: []byte(`{"seq":7}`)}
	if err := notifier.Deliver(context.Background(), delivery); err != nil {
		t.Fatal(err)
	}
	status = http.StatusInternalServerError
	if err := notifier.Deliver(context.Background(), delivery); err == nil {
		t.Fatal("a 500 counted as delivered")
	}
	if Signature("secret", []byte("x")) == Signature("other", []byte("x")) {
		t.Fatal("the signature ignores the secret")
	}
}
