package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"kode-stream/internal/common/models"
)

type Config struct {
	CloudURL          string
	Connect           string
	Repo              string
	HeartbeatInterval time.Duration
	Dialer            *websocket.Dialer
	HTTPClient        *http.Client
	Git               GitMetadata
	Dispatcher        CommandDispatcher
	OnFrame           func(Frame)
}

type Client struct {
	config Config
}

func NewClient(config Config) *Client {
	if config.HeartbeatInterval <= 0 {
		config.HeartbeatInterval = 15 * time.Second
	}
	if config.Dispatcher == nil {
		config.Dispatcher = NoopDispatcher{}
	}
	return &Client{config: config}
}

func (c *Client) Run(ctx context.Context) error {
	token, err := ParseConnectToken(c.config.Connect)
	if err != nil {
		return err
	}
	channelURL, err := ChannelURL(c.config.CloudURL, token)
	if err != nil {
		return err
	}
	dialer := c.config.Dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	connection, response, err := dialer.DialContext(ctx, channelURL, nil)
	if err != nil {
		if response != nil {
			return fmt.Errorf("agent channel dial failed: %s: %w", response.Status, err)
		}
		return fmt.Errorf("agent channel dial failed: %w", err)
	}
	defer connection.Close()
	connection.SetReadLimit(MaxFrameBytes)

	connected, err := c.readFrame(connection, FrameConnected)
	if err != nil {
		return err
	}
	if strings.TrimSpace(c.config.Repo) != "" {
		metadata, err := BuildWorkspaceMetadata(c.config.Repo, c.config.Git)
		if err != nil {
			return err
		}
		if err := PublishWorkspaceMetadata(ctx, c.config.HTTPClient, c.config.CloudURL, token, metadata); err != nil {
			return err
		}
	}

	var writeMu sync.Mutex
	errc := make(chan error, 1)
	go func() { errc <- c.readLoop(connection, &writeMu, connected.Agent) }()
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
			return ctx.Err()
		case err := <-errc:
			return err
		case <-ticker.C:
			writeMu.Lock()
			err := connection.WriteJSON(Frame{Type: FrameHeartbeat})
			writeMu.Unlock()
			if err != nil {
				return err
			}
		}
	}
}

func (c *Client) readLoop(connection *websocket.Conn, writeMu *sync.Mutex, connected models.CloudAgent) error {
	for {
		var frame Frame
		if err := connection.ReadJSON(&frame); err != nil {
			return err
		}
		if err := ValidateFrame(frame, connected.ID, connected.UserID); err != nil {
			return err
		}
		c.emit(frame)
		if frame.Type == FrameCommand {
			result := c.config.Dispatcher.Dispatch(frame.Command)
			writeMu.Lock()
			err := connection.WriteJSON(Frame{Type: FrameResult, Result: &result})
			writeMu.Unlock()
			if err != nil {
				return err
			}
		}
	}
}

func (c *Client) readFrame(connection *websocket.Conn, expected string) (Frame, error) {
	var frame Frame
	if err := connection.ReadJSON(&frame); err != nil {
		return Frame{}, err
	}
	c.emit(frame)
	if frame.Type != expected {
		data, _ := json.Marshal(frame)
		return Frame{}, fmt.Errorf("expected %s frame, got %s", expected, data)
	}
	return frame, nil
}

func (c *Client) emit(frame Frame) {
	if c.config.OnFrame != nil {
		c.config.OnFrame(frame)
	}
}

func apiURL(cloudURL, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(cloudURL))
	if err != nil || parsed.Host == "" {
		return "", errors.New("cloud URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("cloud URL scheme %q is unsupported", parsed.Scheme)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}
