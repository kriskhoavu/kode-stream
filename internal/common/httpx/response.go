package httpx

// Package httpx provides shared HTTP response encoding.

import (
	"encoding/json"
	"net/http"
	"strings"
)

// WriteJSON writes a JSON response using the provided HTTP status.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteError writes the common API error envelope. The hint function may be nil.
func WriteError(w http.ResponseWriter, status int, message string, hint func(string) string) {
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	payload := map[string]string{"error": message}
	if hint != nil {
		if recoveryHint := hint(message); recoveryHint != "" {
			payload["recoveryHint"] = recoveryHint
		}
	}
	WriteJSON(w, status, payload)
}
