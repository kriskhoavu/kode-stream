package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"kode-stream/internal/harness/ports"
)

type Notifier struct {
	URL        string
	Secret     string
	HTTPClient *http.Client
}

func New(url, secret string) *Notifier {
	return &Notifier{URL: url, Secret: secret, HTTPClient: &http.Client{Timeout: 10 * time.Second}}
}

func Signature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (n *Notifier) Deliver(ctx context.Context, delivery ports.Delivery) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(delivery.Body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Harness-Event-Id", delivery.EventID)
	request.Header.Set("X-Harness-Attempt", strconv.Itoa(delivery.Attempt))
	request.Header.Set("X-Harness-Signature", Signature(n.Secret, delivery.Body))
	response, err := n.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("the webhook receiver is unavailable: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("the webhook receiver answered %d", response.StatusCode)
	}
	return nil
}
