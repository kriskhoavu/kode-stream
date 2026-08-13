package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	appaisession "kode-stream/internal/ai"
	"kode-stream/internal/common/models"
	"kode-stream/internal/system"
)

const maxAIMutationBodyBytes int64 = 1 << 20
const maxTerminalFrameBytes int64 = 64 << 10
const maxTerminalInputBytes = 16 << 10
const maxTerminalDimension uint16 = 500
const maxTerminalInputBytesPerSecond = 64 << 10

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
	if !decodeLimitedJSON(w, r, &input, maxAIMutationBodyBytes, true) {
		return
	}
	if !validEmbeddedInput(input) {
		writeError(w, http.StatusBadRequest, "AI launch input exceeds the allowed limits")
		return
	}
	if !a.authorizeItemWorkspace(w, r, r.PathValue("id")) {
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
	if !decodeLimitedJSON(w, r, &input, maxAIMutationBodyBytes, true) {
		return
	}
	if !validEmbeddedInput(input) {
		writeError(w, http.StatusBadRequest, "AI launch input exceeds the allowed limits")
		return
	}
	if !a.authorizeWorkspace(w, r, r.PathValue("id")) {
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
	if !a.authorizeWorkspace(w, r, session.WorkspaceID) {
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (a *aiController) embeddedAISessionGrant(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil || a.aiSessions.EmbeddedManager() == nil {
		writeError(w, http.StatusServiceUnavailable, "embedded AI sessions are unavailable")
		return
	}
	manager := a.aiSessions.EmbeddedManager()
	session, err := manager.Get(r.PathValue("sessionId"))
	if err != nil || !a.authorizeWorkspace(w, r, session.WorkspaceID) {
		if err != nil {
			writeError(w, http.StatusNotFound, "session not found")
		}
		return
	}
	grant, err := manager.IssueGrant(r.PathValue("sessionId"))
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
	manager := a.aiSessions.EmbeddedManager()
	existing, err := manager.Get(r.PathValue("sessionId"))
	if err != nil || !a.authorizeWorkspace(w, r, existing.WorkspaceID) {
		if err != nil {
			writeError(w, http.StatusNotFound, "session not found")
		}
		return
	}
	session, err := manager.Cancel(r.PathValue("sessionId"))
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
	session, err := manager.Get(id)
	if err != nil || !a.authorizeWorkspace(w, r, session.WorkspaceID) {
		if err != nil {
			writeError(w, http.StatusNotFound, "session not found")
		}
		return
	}
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
	connection.SetReadLimit(maxTerminalFrameBytes)
	var writeMu sync.Mutex
	writeChannel := func(payload channelOutput) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return connection.WriteJSON(payload)
	}
	output, buffered, unsubscribe, err := manager.Subscribe(id)
	if err != nil {
		return
	}
	defer unsubscribe()
	if len(buffered) > 0 {
		_ = writeChannel(channelOutput{Type: "output", Data: base64.StdEncoding.EncodeToString(buffered), Encoding: "base64"})
	}
	state, _ := manager.Get(id)
	_ = writeChannel(channelOutput{Type: "state", State: state.State, ExitCode: state.ExitCode})
	done := make(chan struct{})
	defer close(done)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		windowStarted := time.Now()
		windowBytes := 0
		for {
			messageType, reader, err := connection.NextReader()
			if err != nil || messageType != websocket.TextMessage {
				return
			}
			var message channelInput
			decoder := json.NewDecoder(io.LimitReader(reader, maxTerminalFrameBytes))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&message); err != nil {
				_ = writeChannel(channelOutput{Type: "error", Message: "invalid terminal message"})
				return
			}
			var extra any
			if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
				_ = writeChannel(channelOutput{Type: "error", Message: "invalid terminal message"})
				return
			}
			switch message.Type {
			case "input":
				decoded, err := base64.StdEncoding.DecodeString(message.Data)
				if err != nil || len(decoded) == 0 || len(decoded) > maxTerminalInputBytes {
					_ = writeChannel(channelOutput{Type: "error", Message: "invalid terminal input"})
					return
				}
				if time.Since(windowStarted) >= time.Second {
					windowStarted, windowBytes = time.Now(), 0
				}
				windowBytes += len(decoded)
				if windowBytes > maxTerminalInputBytesPerSecond {
					_ = writeChannel(channelOutput{Type: "error", Message: "terminal input rate exceeded"})
					return
				}
				if err := manager.Write(id, decoded); err != nil {
					_ = writeChannel(channelOutput{Type: "error", Message: "terminal input rejected"})
					return
				}
			case "resize":
				if message.Columns == 0 || message.Rows == 0 || message.Columns > maxTerminalDimension || message.Rows > maxTerminalDimension {
					_ = writeChannel(channelOutput{Type: "error", Message: "invalid terminal size"})
					return
				}
				if err := manager.Resize(id, message.Columns, message.Rows); err != nil {
					_ = writeChannel(channelOutput{Type: "error", Message: "terminal resize rejected"})
					return
				}
			case "cancel":
				if _, err := manager.Cancel(id); err != nil {
					_ = writeChannel(channelOutput{Type: "error", Message: "terminal cancel rejected"})
					return
				}
			case "heartbeat":
				if message.Data != "" || message.Columns != 0 || message.Rows != 0 {
					_ = writeChannel(channelOutput{Type: "error", Message: "invalid terminal message"})
					return
				}
			default:
				_ = writeChannel(channelOutput{Type: "error", Message: "unknown terminal message"})
				return
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
			if err := writeChannel(channelOutput{Type: "output", Data: base64.StdEncoding.EncodeToString(data), Encoding: "base64"}); err != nil {
				return
			}
		case <-ticker.C:
			current, err := manager.Get(id)
			if err != nil {
				return
			}
			if current.State != lastState {
				if err := writeChannel(channelOutput{Type: "state", State: current.State, ExitCode: current.ExitCode}); err != nil {
					return
				}
				lastState = current.State
				if current.State != "running" && current.State != "starting" {
					return
				}
			}
		case <-done:
			return
		case <-readDone:
			return
		}
	}
}

func (a *aiController) aiSessionEligibility(w http.ResponseWriter, r *http.Request) {
	if a.aiSessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AI session launch is unavailable")
		return
	}
	if !a.authorizeItemWorkspace(w, r, r.PathValue("id")) {
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
	if !decodeLimitedJSON(w, r, &input, maxAIMutationBodyBytes, true) {
		return
	}
	if !validLaunchInput(input) {
		writeError(w, http.StatusBadRequest, "AI launch input exceeds the allowed limits")
		return
	}
	if !a.authorizeItemWorkspace(w, r, r.PathValue("id")) {
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
	if !decodeLimitedJSON(w, r, &input, maxAIMutationBodyBytes, true) {
		return
	}
	if !validLaunchInput(input) {
		writeError(w, http.StatusBadRequest, "AI launch input exceeds the allowed limits")
		return
	}
	if !a.authorizeWorkspace(w, r, r.PathValue("id")) {
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
		if !a.authorizeWorkspace(w, r, workspaceID) {
			return
		}
		result, err = a.aiSessions.ProviderCapabilitiesForWorkspaceContext(r.Context(), r.PathValue("id"), workspaceID)
	} else {
		if itemID := strings.TrimSpace(r.URL.Query().Get("itemId")); itemID != "" && !a.authorizeItemWorkspace(w, r, itemID) {
			return
		}
		result, err = a.aiSessions.ProviderCapabilitiesContext(r.Context(), r.PathValue("id"), r.URL.Query().Get("itemId"))
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
	if !decodeLimitedJSON(w, r, &settings, maxAIMutationBodyBytes, true) {
		return
	}
	saved, err := a.aiSessions.Save(settings)
	respond(w, saved, err)
}

// Cloud denials intentionally use the same response as a missing resource.  Session
// IDs are process-local bearer-adjacent identifiers and must not become an oracle.
func (a *aiController) authorizeWorkspace(w http.ResponseWriter, r *http.Request, workspaceID string) bool {
	if a.runtimeConfig.Mode != models.RuntimeModeCloud {
		return true
	}
	session, ok := cloudSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Cloud session is required")
		return false
	}
	if a.cloudWorkspaces == nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return false
	}
	if _, found, err := a.cloudWorkspaces.Get(r.Context(), session.User.ID, strings.TrimSpace(workspaceID)); err != nil || !found {
		writeError(w, http.StatusNotFound, "workspace not found")
		return false
	}
	return true
}

func (a *aiController) authorizeItemWorkspace(w http.ResponseWriter, r *http.Request, itemID string) bool {
	if a.runtimeConfig.Mode != models.RuntimeModeCloud {
		return true
	}
	workspaceID, err := a.aiSessions.ItemWorkspace(itemID)
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return false
	}
	return a.authorizeWorkspace(w, r, workspaceID)
}

func validLaunchInput(input appaisession.LaunchInput) bool {
	return validAIStrings(input.Provider, input.Terminal, input.ContextMode, input.PresetID, input.PromptDraft, input.CustomPrompt, input.ContextPath) && validCapabilitySelection(input.SelectedSkills, input.SelectedAgents)
}

func validEmbeddedInput(input appaisession.EmbeddedInput) bool {
	return input.Columns >= 20 && input.Columns <= maxTerminalDimension && input.Rows >= 5 && input.Rows <= 200 && validAIStrings(input.Provider, input.ContextMode, input.PresetID, input.PromptDraft, input.CustomPrompt, input.ContextPath, input.ExpectedWorkspaceID, input.ExpectedBranch, input.ObservedCommit, input.IdempotencyKey) && validCapabilitySelection(input.SelectedSkills, input.SelectedAgents)
}

func validAIStrings(values ...string) bool {
	for _, value := range values {
		if len(value) > 64<<10 {
			return false
		}
	}
	return true
}

func validCapabilitySelection(groups ...[]string) bool {
	for _, group := range groups {
		if len(group) > 64 {
			return false
		}
		for _, value := range group {
			if len(value) == 0 || len(value) > 512 {
				return false
			}
		}
	}
	return true
}
