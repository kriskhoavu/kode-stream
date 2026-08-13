package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	appaisession "kode-stream/internal/ai"
)

func TestEmbeddedTerminalChannelRejectsAbusiveFramesWithoutChangingProcess(t *testing.T) {
	manager := appaisession.NewTerminalManager(appaisession.Config{})
	t.Cleanup(func() { _ = manager.Close() })
	session, grant, err := manager.Start(appaisession.StartRequest{ID: "channel-limits", WorkspaceID: "workspace", Executable: "/bin/sh", Args: []string{"-c", "sleep 10"}, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(Dependencies{AISessions: appaisession.New(nil).ConfigureEmbedded(manager)}).Routes())
	t.Cleanup(server.Close)
	for _, frame := range []struct {
		kind int
		data []byte
	}{{websocket.BinaryMessage, []byte("binary")}, {websocket.TextMessage, []byte(`{"type":"input","data":"%%%"}`)}, {websocket.TextMessage, []byte(`{"type":"resize","columns":1,"rows":1}`)}} {
		endpoint, _ := url.Parse(server.URL)
		endpoint.Scheme = "ws"
		endpoint.Path = "/api/ai/sessions/" + session.ID + "/channel"
		query := endpoint.Query()
		query.Set("token", grant.Token)
		endpoint.RawQuery = query.Encode()
		header := http.Header{"Origin": []string{"http://" + endpoint.Host}}
		connection, _, dialErr := websocket.DefaultDialer.Dial(endpoint.String(), header)
		if dialErr != nil {
			t.Fatal(dialErr)
		}
		if err := connection.WriteMessage(frame.kind, frame.data); err != nil {
			t.Fatal(err)
		}
		_ = connection.SetReadDeadline(time.Now().Add(time.Second))
		_, _, _ = connection.ReadMessage()
		_ = connection.Close()
		current, getErr := manager.Get(session.ID)
		if getErr != nil || current.State != appaisession.StateRunning {
			t.Fatalf("frame=%q session=%#v err=%v", frame.data, current, getErr)
		}
	}
}
