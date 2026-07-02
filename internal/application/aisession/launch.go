package aisession

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"plan-manager/internal/aisettings"
	"plan-manager/internal/audit"
	"plan-manager/internal/itemindex"
	"plan-manager/internal/models"
	"plan-manager/internal/registry"
	"plan-manager/internal/security/pathguard"
)

type LaunchInput struct {
	Provider    string `json:"provider"`
	Terminal    string `json:"terminal"`
	ContextMode string `json:"contextMode"`
}

type LaunchResult struct {
	Accepted    bool      `json:"accepted"`
	Provider    string    `json:"provider"`
	Terminal    string    `json:"terminal"`
	ContextMode string    `json:"contextMode"`
	StartedAt   time.Time `json:"startedAt"`
}

type Eligibility struct {
	Editable             bool     `json:"editable"`
	CardContextAvailable bool     `json:"cardContextAvailable"`
	Missing              []string `json:"missing"`
}

type LaunchError struct {
	Code string
	Err  error
}

func (e *LaunchError) Error() string { return e.Err.Error() }
func (e *LaunchError) Unwrap() error { return e.Err }

type ProcessRunner interface {
	Start(name string, args []string, dir string) error
}

type execRunner struct{}

func (execRunner) Start(name string, args []string, dir string) error {
	command := exec.Command(name, args...)
	command.Dir = dir
	return command.Start()
}

type launchDependencies struct {
	registry   *registry.Registry
	index      *itemindex.Index
	audit      *audit.Store
	wrapperDir string
	runner     ProcessRunner
	now        func() time.Time
}

func (s *Service) ConfigureLaunch(reg *registry.Registry, index *itemindex.Index, auditStore *audit.Store, wrapperDir string) *Service {
	s.launch = &launchDependencies{registry: reg, index: index, audit: auditStore, wrapperDir: wrapperDir, runner: execRunner{}, now: time.Now}
	return s
}

func (s *Service) Eligibility(itemID string) (Eligibility, error) {
	if s.launch == nil || s.launch.registry == nil || s.launch.index == nil {
		return Eligibility{}, launchError("launch_failed", "AI session launch is unavailable")
	}
	item, found, err := s.launch.index.Get(itemID)
	if err != nil {
		return Eligibility{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return Eligibility{}, launchError("item_not_found", "item not found")
	}
	workspace, found, err := s.launch.registry.Get(item.WorkspaceID)
	if err != nil {
		return Eligibility{}, launchErrorWith("launch_failed", err)
	}
	if !found {
		return Eligibility{}, launchError("workspace_not_found", "workspace not found")
	}
	result := Eligibility{Editable: item.SourceMode != "snapshot" && item.Editable, Missing: []string{}}
	if !result.Editable {
		result.Missing = append(result.Missing, "editable working-tree item")
		return result, nil
	}
	_, err = pathguard.SafeJoin(workspace.Path, item.ItemPath)
	if err != nil {
		result.Editable = false
		result.Missing = append(result.Missing, "valid item path")
		return result, nil
	}
	result.CardContextAvailable = true
	return result, nil
}

func (s *Service) Launch(itemID string, input LaunchInput) (result LaunchResult, err error) {
	started := time.Now()
	workspaceID := ""
	defer func() {
		if s.launch == nil || s.launch.audit == nil {
			return
		}
		status := models.AuditStatusSuccess
		message := "External AI session launched"
		errorText := ""
		if err != nil {
			status = models.AuditStatusBlocked
			message = "External AI session launch blocked"
			errorText = err.Error()
			var launchErr *LaunchError
			if errors.As(err, &launchErr) && launchErr.Code == "launch_failed" {
				status = models.AuditStatusFailed
				message = "External AI session launch failed"
			}
		}
		_, _ = s.launch.audit.Append(models.AuditEvent{
			WorkspaceID: workspaceID, ItemID: itemID, Operation: "ai_session_launch",
			Status: status, Message: message, DurationMS: time.Since(started).Milliseconds(), Error: errorText,
		})
	}()
	if s.launch == nil || s.launch.registry == nil || s.launch.index == nil {
		return LaunchResult{}, launchError("launch_failed", "AI session launch is unavailable")
	}
	item, found, getErr := s.launch.index.Get(itemID)
	if getErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", getErr)
	}
	if !found {
		return LaunchResult{}, launchError("item_not_found", "item not found")
	}
	workspaceID = item.WorkspaceID
	workspace, found, getErr := s.launch.registry.Get(item.WorkspaceID)
	if getErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", getErr)
	}
	if !found {
		return LaunchResult{}, launchError("workspace_not_found", "workspace not found")
	}
	contextMode := strings.TrimSpace(input.ContextMode)
	if contextMode != "workspace_only" && contextMode != "card_context" {
		return LaunchResult{}, launchError("invalid_context_mode", "contextMode must be workspace_only or card_context")
	}
	if contextMode == "card_context" {
		if item.SourceMode == "snapshot" || !item.Editable {
			return LaunchResult{}, launchError("item_not_editable", "context-based AI sessions require an editable working-tree item")
		}
		_, joinErr := pathguard.SafeJoin(workspace.Path, item.ItemPath)
		if joinErr != nil {
			return LaunchResult{}, launchError("item_not_editable", "item path is outside the workspace")
		}
	}
	settings, settingsErr := s.Settings()
	if settingsErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", settingsErr)
	}
	providerID := strings.TrimSpace(input.Provider)
	terminalID := strings.TrimSpace(input.Terminal)
	provider, ok := settings.Providers[providerID]
	if !ok || !provider.Enabled {
		return LaunchResult{}, launchError("ai_provider_missing", "selected AI provider is unavailable")
	}
	terminal, ok := settings.Terminals[terminalID]
	if !ok || !terminal.Enabled {
		return LaunchResult{}, launchError("terminal_missing", "selected terminal is unavailable")
	}
	if !s.detect(provider.Executable).Detected {
		return LaunchResult{}, launchError("ai_provider_missing", "selected AI provider executable was not found")
	}
	if !s.detect(terminal.Executable).Detected {
		return LaunchResult{}, launchError("terminal_missing", "selected terminal executable was not found")
	}
	values := map[string]string{
		"workspace": workspace.Path, "contextFile": item.ItemPath, "itemPath": item.ItemPath,
		"identifier": item.Identifier, "contextMode": contextMode, "intent": contextMode,
	}
	providerName := expand(provider.Executable, values)
	providerArgs := []string{}
	if contextMode == "card_context" {
		providerArgs = expandAll(provider.Args, values)
	}
	terminalArgs := expandAll(terminal.Args, values)
	if startErr := s.startTerminal(terminalID, terminal, terminalArgs, workspace.Path, providerName, providerArgs); startErr != nil {
		return LaunchResult{}, launchErrorWith("launch_failed", startErr)
	}
	return LaunchResult{Accepted: true, Provider: providerID, Terminal: terminalID, ContextMode: contextMode, StartedAt: s.launch.now().UTC()}, nil
}

func (s *Service) startTerminal(id string, terminal aisettings.LaunchTemplate, terminalArgs []string, workspace, provider string, providerArgs []string) error {
	terminalExecutable := s.detect(terminal.Executable).Executable
	providerExecutable := s.detect(provider).Executable
	if id == "wezterm" {
		args := append(terminalArgs, "start", "--cwd", workspace, "--", providerExecutable)
		return s.launch.runner.Start(terminalExecutable, append(args, providerArgs...), workspace)
	}
	if s.goos != "darwin" || (id != "terminal" && id != "iterm2") {
		return launchError("terminal_missing", "selected terminal has no launch adapter on this platform")
	}
	wrapper, err := writeWrapper(s.launch.wrapperDir, workspace, providerExecutable, providerArgs)
	if err != nil {
		return err
	}
	if err := s.launch.runner.Start("/usr/bin/open", []string{"-a", terminalExecutable, wrapper}, workspace); err != nil {
		_ = os.Remove(wrapper)
		return err
	}
	return nil
}

func writeWrapper(wrapperDir, workspace, executable string, args []string) (string, error) {
	if err := os.MkdirAll(wrapperDir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(wrapperDir, "launch-"+randomID()+".command")
	command := "#!/bin/sh\ncd -- " + shellQuote(workspace) + " || exit 1\nself=$0\nrm -f -- \"$self\"\nexec " + shellQuote(executable)
	for _, arg := range args {
		command += " " + shellQuote(arg)
	}
	command += "\n"
	if err := writeAtomic(path, []byte(command), 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ai-session-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func expand(value string, values map[string]string) string {
	for key, replacement := range values {
		value = strings.ReplaceAll(value, "{"+key+"}", replacement)
	}
	return value
}

func expandAll(values []string, replacements map[string]string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = expand(value, replacements)
	}
	return result
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func randomID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func launchError(code, message string) error {
	return &LaunchError{Code: code, Err: errors.New(message)}
}
func launchErrorWith(code string, err error) error { return &LaunchError{Code: code, Err: err} }
