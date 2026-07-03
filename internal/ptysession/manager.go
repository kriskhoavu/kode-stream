package ptysession

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	StateStarting  = "starting"
	StateRunning   = "running"
	StateExited    = "exited"
	StateCancelled = "cancelled"
	StateFailed    = "failed"
)

var ErrNotFound = errors.New("session not found")
var ErrUnauthorized = errors.New("invalid or expired session grant")
var ErrLimit = errors.New("embedded session limit reached")

type Config struct {
	MaxSessions int
	BufferBytes int
	GracePeriod time.Duration
	GrantTTL    time.Duration
}

type Session struct {
	ID          string    `json:"id"`
	ItemID      string    `json:"itemId"`
	WorkspaceID string    `json:"workspaceId"`
	Provider    string    `json:"provider"`
	Intent      string    `json:"intent"`
	State       string    `json:"state"`
	StartedAt   time.Time `json:"startedAt"`
	ExitCode    *int      `json:"exitCode,omitempty"`
}

type Grant struct {
	SessionID string    `json:"sessionId"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type StartRequest struct {
	ItemID, WorkspaceID, Provider, Intent, Executable, Dir string
	Args                                                   []string
	Columns, Rows                                          uint16
}

type managed struct {
	mu           sync.Mutex
	info         Session
	grantHash    [32]byte
	grantExpires time.Time
	file         *os.File
	command      *exec.Cmd
	buffer       []byte
	subs         map[chan []byte]struct{}
	disconnect   *time.Timer
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*managed
	config   Config
	closed   bool
}

func New(config Config) *Manager {
	if config.MaxSessions <= 0 {
		config.MaxSessions = 4
	}
	if config.BufferBytes <= 0 {
		config.BufferBytes = 256 * 1024
	}
	if config.GracePeriod <= 0 {
		config.GracePeriod = 15 * time.Second
	}
	if config.GrantTTL <= 0 {
		config.GrantTTL = time.Minute
	}
	return &Manager{sessions: map[string]*managed{}, config: config}
}

func (m *Manager) Start(request StartRequest) (Session, Grant, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Session{}, Grant{}, errors.New("session manager is closed")
	}
	active := 0
	for _, session := range m.sessions {
		state := session.snapshot().State
		if state == StateStarting || state == StateRunning {
			active++
		}
	}
	if active >= m.config.MaxSessions {
		m.mu.Unlock()
		return Session{}, Grant{}, ErrLimit
	}
	id, err := random(24)
	if err != nil {
		m.mu.Unlock()
		return Session{}, Grant{}, err
	}
	token, err := random(32)
	if err != nil {
		m.mu.Unlock()
		return Session{}, Grant{}, err
	}
	now := time.Now().UTC()
	s := &managed{info: Session{ID: id, ItemID: request.ItemID, WorkspaceID: request.WorkspaceID, Provider: request.Provider, Intent: request.Intent, State: StateStarting, StartedAt: now}, grantHash: hash(token), grantExpires: now.Add(m.config.GrantTTL), subs: map[chan []byte]struct{}{}}
	m.sessions[id] = s
	m.mu.Unlock()

	command := exec.Command(request.Executable, request.Args...)
	command.Dir = request.Dir
	columns, rows := request.Columns, request.Rows
	if columns == 0 {
		columns = 80
	}
	if rows == 0 {
		rows = 24
	}
	file, err := pty.StartWithSize(command, &pty.Winsize{Cols: columns, Rows: rows})
	if err != nil {
		s.mu.Lock()
		s.info.State = StateFailed
		s.mu.Unlock()
		return s.snapshot(), Grant{}, err
	}
	s.mu.Lock()
	s.file, s.command, s.info.State = file, command, StateRunning
	s.mu.Unlock()
	go m.read(s)
	go m.wait(s)
	return s.snapshot(), Grant{SessionID: id, Token: token, ExpiresAt: s.grantExpires}, nil
}

func (m *Manager) Get(id string) (Session, error) {
	s := m.lookup(id)
	if s == nil {
		return Session{}, ErrNotFound
	}
	return s.snapshot(), nil
}

func (m *Manager) Authenticate(id, token string) error {
	s := m.lookup(id)
	if s == nil {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	actual := hash(token)
	if time.Now().After(s.grantExpires) || subtle.ConstantTimeCompare(actual[:], s.grantHash[:]) != 1 {
		return ErrUnauthorized
	}
	return nil
}

func (m *Manager) Subscribe(id string) (<-chan []byte, []byte, func(), error) {
	s := m.lookup(id)
	if s == nil {
		return nil, nil, nil, ErrNotFound
	}
	ch := make(chan []byte, 32)
	s.mu.Lock()
	if s.disconnect != nil {
		s.disconnect.Stop()
		s.disconnect = nil
	}
	s.subs[ch] = struct{}{}
	buffer := append([]byte(nil), s.buffer...)
	s.mu.Unlock()
	return ch, buffer, func() { m.unsubscribe(s, ch) }, nil
}

func (m *Manager) Write(id string, data []byte) error {
	s := m.lookup(id)
	if s == nil {
		return ErrNotFound
	}
	s.mu.Lock()
	file := s.file
	running := s.info.State == StateRunning
	s.mu.Unlock()
	if !running || file == nil {
		return errors.New("session is not running")
	}
	_, err := file.Write(data)
	return err
}

func (m *Manager) Resize(id string, columns, rows uint16) error {
	if columns < 20 || columns > 500 || rows < 5 || rows > 200 {
		return errors.New("terminal size is outside allowed limits")
	}
	s := m.lookup(id)
	if s == nil {
		return ErrNotFound
	}
	s.mu.Lock()
	file := s.file
	s.mu.Unlock()
	if file == nil {
		return errors.New("session is not running")
	}
	return pty.Setsize(file, &pty.Winsize{Cols: columns, Rows: rows})
}

func (m *Manager) Cancel(id string) (Session, error) { return m.stop(id, StateCancelled) }

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_, _ = m.stop(id, StateCancelled)
	}
	return nil
}

func (m *Manager) stop(id, state string) (Session, error) {
	s := m.lookup(id)
	if s == nil {
		return Session{}, ErrNotFound
	}
	s.mu.Lock()
	if s.info.State == StateRunning || s.info.State == StateStarting {
		s.info.State = state
		if s.command != nil && s.command.Process != nil {
			_ = syscall.Kill(-s.command.Process.Pid, syscall.SIGKILL)
		}
		if s.file != nil {
			_ = s.file.Close()
		}
	}
	s.mu.Unlock()
	return s.snapshot(), nil
}

func (m *Manager) read(s *managed) {
	buffer := make([]byte, 32*1024)
	for {
		n, err := s.file.Read(buffer)
		if n > 0 {
			m.publish(s, buffer[:n])
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
			}
			return
		}
	}
}

func (m *Manager) wait(s *managed) {
	err := s.command.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.info.State == StateRunning {
		s.info.State = StateExited
		code := 0
		if err != nil {
			code = s.command.ProcessState.ExitCode()
		}
		s.info.ExitCode = &code
	}
}

func (m *Manager) publish(s *managed, data []byte) {
	copyData := append([]byte(nil), data...)
	s.mu.Lock()
	s.buffer = append(s.buffer, copyData...)
	if len(s.buffer) > m.config.BufferBytes {
		s.buffer = append([]byte(nil), s.buffer[len(s.buffer)-m.config.BufferBytes:]...)
	}
	for subscriber := range s.subs {
		select {
		case subscriber <- copyData:
		default:
		}
	}
	s.mu.Unlock()
}

func (m *Manager) unsubscribe(s *managed, ch chan []byte) {
	s.mu.Lock()
	if _, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(ch)
	}
	if len(s.subs) == 0 && s.info.State == StateRunning {
		s.disconnect = time.AfterFunc(m.config.GracePeriod, func() { _, _ = m.stop(s.info.ID, StateCancelled) })
	}
	s.mu.Unlock()
}

func (m *Manager) lookup(id string) *managed {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}
func (s *managed) snapshot() Session { s.mu.Lock(); defer s.mu.Unlock(); return s.info }
func random(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
func hash(value string) [32]byte { return sha256.Sum256([]byte(value)) }
