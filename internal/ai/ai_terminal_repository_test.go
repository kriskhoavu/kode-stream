package ai

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestSessionStreamsInputOutputAndReconnectBuffer(t *testing.T) {
	manager := NewTerminalManager(Config{GracePeriod: time.Second})
	t.Cleanup(func() { _ = manager.Close() })
	session, grant, err := manager.Start(StartRequest{ItemIdentifier: "PM-020", ItemTitle: "Embedded terminal", Executable: "/bin/sh", Args: []string{"-c", "read line; printf 'reply:%s' \"$line\""}, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if session.State != StateRunning || grant.Token == "" {
		t.Fatalf("session=%#v grant=%#v", session, grant)
	}
	if session.ItemIdentifier != "PM-020" || session.ItemTitle != "Embedded terminal" {
		t.Fatalf("session metadata=%#v", session)
	}
	if err := manager.Authenticate(session.ID, grant.Token); err != nil {
		t.Fatal(err)
	}
	if err := manager.Authenticate(session.ID, "wrong"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("error=%v", err)
	}
	output, _, disconnect, err := manager.Subscribe(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Write(session.ID, []byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	var received []byte
	deadline := time.After(3 * time.Second)
	for !bytes.Contains(received, []byte("reply:hello")) {
		select {
		case data := <-output:
			received = append(received, data...)
		case <-deadline:
			t.Fatalf("output=%q", received)
		}
	}
	disconnect()
	_, buffered, disconnectAgain, err := manager.Subscribe(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer disconnectAgain()
	if !bytes.Contains(buffered, []byte("reply:hello")) {
		t.Fatalf("buffer=%q", buffered)
	}
}

func TestSessionEnforcesLimitAndCancellation(t *testing.T) {
	manager := NewTerminalManager(Config{MaxSessions: 1})
	t.Cleanup(func() { _ = manager.Close() })
	first, _, err := manager.Start(StartRequest{Executable: "/bin/sh", Args: []string{"-c", "sleep 10"}, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.Start(StartRequest{Executable: "/bin/sh", Args: []string{"-c", "true"}, Dir: t.TempDir()}); !errors.Is(err, ErrLimit) {
		t.Fatalf("error=%v", err)
	}
	cancelled, err := manager.Cancel(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.State != StateCancelled {
		t.Fatalf("state=%s", cancelled.State)
	}
}

func TestDisconnectedSessionExpiresAfterGracePeriod(t *testing.T) {
	manager := NewTerminalManager(Config{GracePeriod: 20 * time.Millisecond})
	t.Cleanup(func() { _ = manager.Close() })
	session, _, err := manager.Start(StartRequest{Executable: "/bin/sh", Args: []string{"-c", "sleep 10"}, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	_, _, disconnect, err := manager.Subscribe(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	disconnect()
	time.Sleep(100 * time.Millisecond)
	current, err := manager.Get(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.State != StateCancelled {
		t.Fatalf("state=%s", current.State)
	}
}

func TestResizeRejectsUnsafeDimensions(t *testing.T) {
	manager := NewTerminalManager(Config{})
	defer manager.Close()
	session, _, err := manager.Start(StartRequest{Executable: "/bin/sh", Args: []string{"-c", "sleep 10"}, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Resize(session.ID, 10, 2); err == nil {
		t.Fatal("expected invalid size")
	}
	if err := manager.Resize(session.ID, 120, 40); err != nil {
		t.Fatal(err)
	}
}
