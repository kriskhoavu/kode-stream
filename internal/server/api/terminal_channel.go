package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	appaisession "kode-stream/internal/ai"
)

const (
	terminalReadTimeout  = time.Minute
	terminalWriteTimeout = 10 * time.Second
)

// terminalChannel carries one embedded session over one WebSocket: the
// session's output and state out, a viewer's input, resizes and cancel in.
type terminalChannel struct {
	id            string
	manager       *appaisession.Manager
	connection    *websocket.Conn
	writeMu       sync.Mutex
	windowStarted time.Time
	windowBytes   int
}

func newTerminalChannel(id string, manager *appaisession.Manager, connection *websocket.Conn) *terminalChannel {
	connection.SetReadLimit(maxTerminalFrameBytes)
	_ = connection.SetReadDeadline(time.Now().Add(terminalReadTimeout))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(terminalReadTimeout))
	})
	return &terminalChannel{id: id, manager: manager, connection: connection, windowStarted: time.Now()}
}

func (c *terminalChannel) write(payload channelOutput) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.connection.SetWriteDeadline(time.Now().Add(terminalWriteTimeout)); err != nil {
		return err
	}
	return c.connection.WriteJSON(payload)
}

func (c *terminalChannel) writeOutput(data []byte) error {
	return c.write(channelOutput{Type: "output", Data: base64.StdEncoding.EncodeToString(data), Encoding: "base64"})
}

func (c *terminalChannel) writeState(session appaisession.Session) error {
	return c.write(channelOutput{Type: "state", State: session.State, ExitCode: session.ExitCode})
}

func (c *terminalChannel) fail(message string) {
	_ = c.write(channelOutput{Type: "error", Message: message})
}

// readLoop handles the viewer's messages until one closes the channel or
// the connection drops, then closes done.
func (c *terminalChannel) readLoop(done chan<- struct{}) {
	defer close(done)
	for {
		message, ok := c.readMessage()
		if !ok || !c.handle(message) {
			return
		}
	}
}

// readMessage reads exactly one JSON message from one text frame.
func (c *terminalChannel) readMessage() (channelInput, bool) {
	messageType, reader, err := c.connection.NextReader()
	if err != nil || messageType != websocket.TextMessage {
		return channelInput{}, false
	}
	var message channelInput
	decoder := json.NewDecoder(io.LimitReader(reader, maxTerminalFrameBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&message); err != nil {
		c.fail("invalid terminal message")
		return channelInput{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		c.fail("invalid terminal message")
		return channelInput{}, false
	}
	return message, true
}

// handle applies one message and says whether the channel stays open.
func (c *terminalChannel) handle(message channelInput) bool {
	switch message.Type {
	case "input":
		return c.input(message.Data)
	case "resize":
		c.resize(message.Columns, message.Rows)
		return true
	case "cancel":
		if _, err := c.manager.Cancel(c.id); err != nil {
			c.fail("terminal cancel rejected")
			return false
		}
		return true
	case "heartbeat":
		if message.Data != "" || message.Columns != 0 || message.Rows != 0 {
			c.fail("invalid terminal message")
			return false
		}
		return true
	default:
		c.fail("unknown terminal message")
		return false
	}
}

func (c *terminalChannel) input(data string) bool {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(decoded) == 0 || len(decoded) > maxTerminalInputBytes {
		c.fail("invalid terminal input")
		return false
	}
	if time.Since(c.windowStarted) >= time.Second {
		c.windowStarted, c.windowBytes = time.Now(), 0
	}
	c.windowBytes += len(decoded)
	if c.windowBytes > maxTerminalInputBytesPerSecond {
		c.fail("terminal input rate exceeded")
		return false
	}
	if err := c.manager.Write(c.id, decoded); err != nil {
		c.fail("terminal input rejected")
		return false
	}
	return true
}

// resize refuses a size outside the PTY's limits but keeps the channel open:
// a viewer briefly laid out at a sliver must not lose the session, which the
// manager cancels once nobody is watching.
func (c *terminalChannel) resize(columns, rows uint16) {
	if columns == 0 || rows == 0 || columns > maxTerminalDimension || rows > maxTerminalDimension {
		c.fail("invalid terminal size")
		return
	}
	if err := c.manager.Resize(c.id, columns, rows); err != nil {
		c.fail("terminal resize rejected")
	}
}

// pump sends output and state changes until the session ends, the
// connection fails or the reader stops.
func (c *terminalChannel) pump(output <-chan []byte, lastState string, readDone <-chan struct{}) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	for {
		select {
		case data, ok := <-output:
			if !ok || c.writeOutput(data) != nil {
				return
			}
		case <-ticker.C:
			current, err := c.manager.Get(c.id)
			if err != nil {
				return
			}
			if current.State == lastState {
				continue
			}
			if c.writeState(current) != nil {
				return
			}
			lastState = current.State
			if current.State != "running" && current.State != "starting" {
				return
			}
		case <-pingTicker.C:
			if c.connection.WriteControl(websocket.PingMessage, nil, time.Now().Add(terminalWriteTimeout)) != nil {
				return
			}
		case <-readDone:
			return
		}
	}
}
