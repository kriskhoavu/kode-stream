package ai

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"kode-stream/internal/filesystem/atomicfile"
)

func writeWrapper(wrapperDir, workspace, executable string, args []string, checkpoint verificationCheckpoint) (string, error) {
	if err := os.MkdirAll(wrapperDir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(wrapperDir, "launch-"+randomID()+".command")
	command := "#!/bin/sh\ncd -- " + shellQuote(workspace) + " || exit 1\nself=$0\nrm -f -- \"$self\"\n" + shellQuote(executable)
	for _, arg := range args {
		command += " " + shellQuote(arg)
	}
	command += "\ncode=$?\n"
	if callback := checkpointCommand(checkpoint); callback != "" {
		command += callback + "\n"
	}
	command += "exit $code\n"
	if err := writeAtomic(path, []byte(command), 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func checkpointCommand(checkpoint verificationCheckpoint) string {
	if strings.TrimSpace(checkpoint.WorkspaceID) == "" {
		return ""
	}
	body, err := json.Marshal(map[string]string{
		"eventType":    "session_completed",
		"profile":      firstNonEmpty(strings.TrimSpace(checkpoint.Profile), "smoke"),
		"provider":     strings.TrimSpace(checkpoint.Provider),
		"sessionId":    strings.TrimSpace(checkpoint.SessionID),
		"terminalMode": firstNonEmpty(strings.TrimSpace(checkpoint.TerminalMode), "external"),
	})
	if err != nil {
		return ""
	}
	urlValue := fmt.Sprintf("http://127.0.0.1:%s/api/workspaces/%s/verification-checkpoints", serverPort(), url.PathEscape(checkpoint.WorkspaceID))
	return "if command -v curl >/dev/null 2>&1; then curl -sS -X POST -H 'Content-Type: application/json' --data " + shellQuote(string(body)) + " " + shellQuote(urlValue) + " >/dev/null 2>&1 || true; fi"
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	return atomicfile.Replace(path, ".ai-session-*", data, mode)
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

func expandProviderArgs(values []string, replacements map[string]string) []string {
	expanded := expandAll(values, replacements)
	if containsPlaceholder(values, "{prompt}") {
		return expanded
	}
	if prompt := strings.TrimSpace(replacements["prompt"]); prompt != "" {
		return append(expanded, prompt)
	}
	return expanded
}

func launchProviderArgs(contextMode string, values []string, replacements map[string]string) []string {
	if strings.TrimSpace(contextMode) == "workspace_only" {
		prompt := strings.TrimSpace(replacements["prompt"])
		if prompt == "" {
			return nil
		}
		return []string{prompt}
	}
	return expandProviderArgs(values, replacements)
}

func containsPlaceholder(values []string, placeholder string) bool {
	for _, value := range values {
		if strings.Contains(value, placeholder) {
			return true
		}
	}
	return false
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
