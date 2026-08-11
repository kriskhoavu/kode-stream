package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	appaisession "kode-stream/internal/ai"
	"kode-stream/internal/common/models"
	"kode-stream/internal/system"
)

type aiController struct {
	aiSessions      *appaisession.Service
	runtimeConfig   system.RuntimeConfig
	cloudWorkspaces *cloudWorkspaceStore
}

func (a *aiController) startEmbeddedAISession(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	var input appaisession.EmbeddedInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.aiSessions.StartEmbedded(r.PathValue("id"), input)
	if err == nil {
		writeJSON(w, http.StatusCreated, result)
		return
	}
	writeEmbeddedLaunchError(w, err)
}

func (a *aiController) startEmbeddedWorkspaceAISession(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	var input appaisession.EmbeddedInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.aiSessions.StartEmbeddedWorkspace(r.PathValue("id"), input)
	if err == nil {
		writeJSON(w, http.StatusCreated, result)
		return
	}
	writeEmbeddedLaunchError(w, err)
}

func writeEmbeddedLaunchError(w http.ResponseWriter, err error) {
	var launchErr *appaisession.LaunchError
	if !errors.As(err, &launchErr) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := http.StatusBadRequest
	switch launchErr.Code {
	case "item_not_found", "workspace_not_found":
		status = http.StatusNotFound
	case "terminal_workspace_mismatch", "terminal_branch_mismatch", "terminal_revision_mismatch":
		status = http.StatusConflict
	case "launch_failed":
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]any{"error": launchErr.Error(), "code": launchErr.Code, "details": launchErr.Details})
}

func (a *aiController) aiSessionRecords(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session records are unavailable")
		return
	}
	workspaceID := strings.TrimSpace(r.URL.Query().Get("workspaceId"))
	if a.runtimeConfig.Mode == models.RuntimeModeCloud {
		session, ok := cloudSessionFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "Cloud session is required")
			return
		}
		if workspaceID == "" {
			writeError(w, http.StatusBadRequest, "workspaceId is required")
			return
		}
		if _, ok, err := a.cloudWorkspaces.Get(r.Context(), session.User.ID, workspaceID); err != nil || !ok {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
	}
	records, err := a.aiSessions.SessionRecords(workspaceID, r.URL.Query().Get("branch"))
	respond(w, records, err)
}

func (a *aiController) embeddedAISession(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil || a.aiSessions.EmbeddedManager() == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	session, err := a.aiSessions.EmbeddedManager().Get(r.PathValue("sessionId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (a *aiController) embeddedAISessionGrant(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil || a.aiSessions.EmbeddedManager() == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	grant, err := a.aiSessions.EmbeddedManager().IssueGrant(r.PathValue("sessionId"))
	if err != nil {
		if errors.Is(err, appaisession.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, grant)
}

func (a *aiController) cancelEmbeddedAISession(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil || a.aiSessions.EmbeddedManager() == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	session, err := a.aiSessions.EmbeddedManager().Cancel(r.PathValue("sessionId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

type channelInput struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	Columns uint16 `json:"columns,omitempty"`
	Rows    uint16 `json:"rows,omitempty"`
}
type channelOutput struct {
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	State    string `json:"state,omitempty"`
	ExitCode *int   `json:"exitCode,omitempty"`
	Message  string `json:"message,omitempty"`
}

func (a *aiController) embeddedAISessionChannel(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil || a.aiSessions.EmbeddedManager() == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	manager := a.aiSessions.EmbeddedManager()
	id := r.PathValue("sessionId")
	if err := manager.Authenticate(id, r.URL.Query().Get("token")); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session grant")
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(request *http.Request) bool {
		return request.Header.Get("Origin") == "http://"+request.Host || request.Header.Get("Origin") == "https://"+request.Host
	}}
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	output, buffered, unsubscribe, err := manager.Subscribe(id)
	if err != nil {
		return
	}
	defer unsubscribe()
	if len(buffered) > 0 {
		_ = connection.WriteJSON(channelOutput{Type: "output", Data: base64.StdEncoding.EncodeToString(buffered), Encoding: "base64"})
	}
	state, _ := manager.Get(id)
	_ = connection.WriteJSON(channelOutput{Type: "state", State: state.State, ExitCode: state.ExitCode})
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			var message channelInput
			if err := connection.ReadJSON(&message); err != nil {
				return
			}
			switch message.Type {
			case "input":
				decoded, err := base64.StdEncoding.DecodeString(message.Data)
				if err == nil {
					_ = manager.Write(id, decoded)
				}
			case "resize":
				_ = manager.Resize(id, message.Columns, message.Rows)
			case "cancel":
				_, _ = manager.Cancel(id)
			case "heartbeat":
			}
		}
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	lastState := state.State
	for {
		select {
		case data, ok := <-output:
			if !ok {
				return
			}
			if err := connection.WriteJSON(channelOutput{Type: "output", Data: base64.StdEncoding.EncodeToString(data), Encoding: "base64"}); err != nil {
				return
			}
		case <-ticker.C:
			current, err := manager.Get(id)
			if err != nil {
				return
			}
			if current.State != lastState {
				if err := connection.WriteJSON(channelOutput{Type: "state", State: current.State, ExitCode: current.ExitCode}); err != nil {
					return
				}
				lastState = current.State
				if current.State != "running" && current.State != "starting" {
					return
				}
			}
		case <-done:
			return
		}
	}
}

func (a *aiController) aiSessionEligibility(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session launch is unavailable")
		return
	}
	result, err := a.aiSessions.Eligibility(r.PathValue("id"))
	if err == nil {
		writeJSON(w, http.StatusOK, result)
		return
	}
	var launchErr *appaisession.LaunchError
	if errors.As(err, &launchErr) && (launchErr.Code == "item_not_found" || launchErr.Code == "workspace_not_found") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": launchErr.Error(), "code": launchErr.Code})
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (a *aiController) launchAISession(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session launch is unavailable")
		return
	}
	var input appaisession.LaunchInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.aiSessions.Launch(r.PathValue("id"), input)
	if err == nil {
		writeJSON(w, http.StatusAccepted, result)
		return
	}
	var launchErr *appaisession.LaunchError
	if !errors.As(err, &launchErr) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := http.StatusBadRequest
	if launchErr.Code == "item_not_found" || launchErr.Code == "workspace_not_found" {
		status = http.StatusNotFound
	} else if launchErr.Code == "launch_failed" {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]string{"error": launchErr.Error(), "code": launchErr.Code})
}

func (a *aiController) launchWorkspaceAISession(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session launch is unavailable")
		return
	}
	var input appaisession.LaunchInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := a.aiSessions.LaunchWorkspace(r.PathValue("id"), input)
	if err == nil {
		writeJSON(w, http.StatusAccepted, result)
		return
	}
	var launchErr *appaisession.LaunchError
	if !errors.As(err, &launchErr) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := http.StatusBadRequest
	if launchErr.Code == "workspace_not_found" {
		status = http.StatusNotFound
	} else if launchErr.Code == "launch_failed" {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]string{"error": launchErr.Error(), "code": launchErr.Code})
}

func (a *aiController) aiCapabilities(w http.ResponseWriter, _ *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
		return
	}
	capabilities, err := a.aiSessions.Capabilities()
	respond(w, capabilities, err)
}

func (a *aiController) aiPresets(w http.ResponseWriter, _ *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
		return
	}
	writeJSON(w, http.StatusOK, a.aiSessions.Presets())
}

func (a *aiController) aiProviderCapabilities(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
		return
	}
	var result appaisession.ProviderCapabilityCatalog
	var err error
	if workspaceID := strings.TrimSpace(r.URL.Query().Get("workspaceId")); workspaceID != "" {
		result, err = a.aiSessions.ProviderCapabilitiesForWorkspace(r.PathValue("id"), workspaceID)
	} else {
		result, err = a.aiSessions.ProviderCapabilities(r.PathValue("id"), r.URL.Query().Get("itemId"))
	}
	if err == nil {
		writeJSON(w, http.StatusOK, result)
		return
	}
	var launchErr *appaisession.LaunchError
	if errors.As(err, &launchErr) && (launchErr.Code == "ai_provider_missing" || launchErr.Code == "item_not_found" || launchErr.Code == "workspace_not_found") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": launchErr.Error(), "code": launchErr.Code})
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (a *aiController) aiSettings(w http.ResponseWriter, _ *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
		return
	}
	settings, err := a.aiSessions.Settings()
	respond(w, settings, err)
}

func (a *aiController) saveAISettings(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session settings are unavailable")
		return
	}
	var settings appaisession.Settings
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	saved, err := a.aiSessions.Save(settings)
	respond(w, saved, err)
}
