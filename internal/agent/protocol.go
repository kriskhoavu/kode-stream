package agent

import "kode-stream/internal/common/models"

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
