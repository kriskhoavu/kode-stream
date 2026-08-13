package agent

import (
	"fmt"
	"strings"

	"kode-stream/internal/common/models"
)

const MaxFrameBytes int64 = 64 << 10

const (
	FrameConnected    = "connected"
	FrameHeartbeat    = "heartbeat"
	FrameHeartbeatAck = "heartbeat_ack"
	FrameCommand      = "command"
	FrameResult       = "result"
)

type Frame struct {
	Type    string                 `json:"type"`
	Agent   models.CloudAgent      `json:"agent,omitempty"`
	Command models.CommandEnvelope `json:"command,omitempty"`
	Result  *CommandResult         `json:"result,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

type CommandResult struct {
	ID      string            `json:"id"`
	OK      bool              `json:"ok"`
	Payload map[string]string `json:"payload,omitempty"`
	Error   string            `json:"error,omitempty"`
}

type CommandDispatcher interface {
	Dispatch(models.CommandEnvelope) CommandResult
}

type NoopDispatcher struct{}

func (NoopDispatcher) Dispatch(command models.CommandEnvelope) CommandResult {
	return CommandResult{ID: command.ID, OK: false, Error: "agent command dispatch is not implemented"}
}

func ValidateFrame(frame Frame, expectedAgentID, expectedUserID string) error {
	switch frame.Type {
	case FrameConnected, FrameHeartbeat, FrameHeartbeatAck, FrameCommand, FrameResult, "error":
	default:
		return fmt.Errorf("agent frame type is invalid")
	}
	if len(frame.Error) > 4<<10 {
		return fmt.Errorf("agent frame error is too large")
	}
	if frame.Type == FrameCommand {
		command := frame.Command
		if command.ID == "" || len(command.ID) > 160 || command.WorkspaceID == "" || len(command.WorkspaceID) > 256 || command.AgentID != expectedAgentID || command.UserID != expectedUserID || !validCapability(command.Capability) || len(command.Payload) > 32 {
			return fmt.Errorf("agent command is invalid")
		}
		for key, value := range command.Payload {
			if strings.TrimSpace(key) == "" || len(key) > 128 || len(value) > 4<<10 {
				return fmt.Errorf("agent command payload is invalid")
			}
		}
	}
	if frame.Type == FrameResult && (frame.Result == nil || frame.Result.ID == "" || len(frame.Result.ID) > 160 || len(frame.Result.Payload) > 32 || len(frame.Result.Error) > 4<<10) {
		return fmt.Errorf("agent command result is invalid")
	}
	return nil
}

func validCapability(value models.Capability) bool {
	switch value {
	case models.CapabilityRead, models.CapabilityWrite, models.CapabilityGit, models.CapabilityTerminal, models.CapabilityAI, models.CapabilityRuntime, models.CapabilityVerification:
		return true
	default:
		return false
	}
}
