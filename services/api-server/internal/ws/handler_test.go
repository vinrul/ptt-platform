package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"ptt-fleet/services/api-server/internal/auth"
	"ptt-fleet/services/api-server/internal/gps"
)

type fakeAccessRepository struct {
	identity Identity
	joinErr  error
}

type fakeGPSRecorder struct {
	location gps.Location
	err      error
}

func (r fakeGPSRecorder) Record(_ context.Context, userID string, update gps.Update) (gps.Location, error) {
	if r.err != nil {
		return gps.Location{}, r.err
	}
	location := r.location
	location.UserID = userID
	location.Lat = update.Lat
	location.Lng = update.Lng
	return location, nil
}

func (r fakeAccessRepository) ActiveIdentity(_ context.Context, userID string) (Identity, error) {
	if r.identity.UserID != userID {
		return Identity{}, ErrUserNotFound
	}
	return r.identity, nil
}

func (r fakeAccessRepository) CanJoinGroup(_ context.Context, _ string, _ string) error {
	return r.joinErr
}

func TestHandlerRejectsInvalidToken(t *testing.T) {
	server, _, _ := newWebSocketTestServer(t, fakeAccessRepository{
		identity: Identity{UserID: "user-1", Username: "field1", Role: "field_user"},
	})
	defer server.Close()

	_, response, err := websocket.DefaultDialer.Dial(websocketURL(server.URL)+"?token=invalid", nil)
	if err == nil {
		t.Fatal("expected invalid token dial to fail")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %#v", response)
	}
}

func TestHandlerSendsReadyBeforePresence(t *testing.T) {
	server, manager, hub := newWebSocketTestServer(t, fakeAccessRepository{
		identity: Identity{UserID: "dispatcher-1", Username: "dispatcher1", Role: "dispatcher"},
	})
	defer server.Close()
	defer hub.CloseAll()

	connection := dialTestConnection(t, server.URL, manager, "dispatcher-1", "dispatcher1", "dispatcher")
	defer connection.Close()

	ready := readEvent(t, connection)
	if ready.Type != "connection.ready" {
		t.Fatalf("expected connection.ready first, got %s", ready.Type)
	}

	presence := readEvent(t, connection)
	if presence.Type != "presence.updated" {
		t.Fatalf("expected presence.updated second, got %s", presence.Type)
	}
}

func TestHandlerValidatesEnvelopeAndJoinsGroup(t *testing.T) {
	server, manager, hub := newWebSocketTestServer(t, fakeAccessRepository{
		identity: Identity{UserID: "user-1", Username: "field1", Role: "field_user"},
	})
	defer server.Close()
	defer hub.CloseAll()

	connection := dialTestConnection(t, server.URL, manager, "user-1", "field1", "field_user")
	defer connection.Close()
	if event := readEvent(t, connection); event.Type != "connection.ready" {
		t.Fatalf("expected connection.ready, got %s", event.Type)
	}

	if err := connection.WriteMessage(websocket.TextMessage, []byte(`{"payload":{}}`)); err != nil {
		t.Fatalf("write invalid event: %v", err)
	}
	if event := readEvent(t, connection); event.Type != "error" {
		t.Fatalf("expected error event, got %s", event.Type)
	}

	join := NewEvent("group.join", "req-join", map[string]any{"groupId": "group-1"})
	if err := connection.WriteJSON(join); err != nil {
		t.Fatalf("write group.join: %v", err)
	}
	joined := readEvent(t, connection)
	if joined.Type != "group.joined" || joined.RequestID != "req-join" {
		t.Fatalf("expected correlated group.joined event, got %#v", joined)
	}
}

func TestHandlerRejectsUnauthorizedGroupJoin(t *testing.T) {
	server, manager, hub := newWebSocketTestServer(t, fakeAccessRepository{
		identity: Identity{UserID: "user-1", Username: "field1", Role: "field_user"},
		joinErr:  ErrGroupNotAllowed,
	})
	defer server.Close()
	defer hub.CloseAll()

	connection := dialTestConnection(t, server.URL, manager, "user-1", "field1", "field_user")
	defer connection.Close()
	_ = readEvent(t, connection)

	if err := connection.WriteJSON(NewEvent("group.join", "req-join", map[string]any{"groupId": "group-1"})); err != nil {
		t.Fatalf("write group.join: %v", err)
	}
	event := readEvent(t, connection)
	if event.Type != "error" {
		t.Fatalf("expected error event, got %s", event.Type)
	}

	var payload ErrorPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	if payload.Code != "forbidden" {
		t.Fatalf("expected forbidden code, got %s", payload.Code)
	}
}

func TestHandlerAcceptsHeartbeat(t *testing.T) {
	server, manager, hub := newWebSocketTestServer(t, fakeAccessRepository{
		identity: Identity{UserID: "user-1", Username: "field1", Role: "field_user"},
	})
	defer server.Close()
	defer hub.CloseAll()

	connection := dialTestConnection(t, server.URL, manager, "user-1", "field1", "field_user")
	defer connection.Close()
	_ = readEvent(t, connection)

	if err := connection.WriteJSON(NewEvent("heartbeat", "", map[string]any{})); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if hub.UserConnectionCount("user-1") != 1 {
		t.Fatal("expected heartbeat connection to remain registered")
	}
}

func newWebSocketTestServer(t *testing.T, repository AccessRepository) (*httptest.Server, *auth.TokenManager, *Hub) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	manager := auth.NewTokenManager("12345678901234567890123456789012", time.Minute)
	hub := NewHub()
	handler := NewHandler(manager, repository, fakeGPSRecorder{
		location: gps.Location{RecordedAt: time.Now().UTC().Format(time.RFC3339)},
	}, hub)
	router := gin.New()
	router.GET("/ws", handler.Connect)

	return httptest.NewServer(router), manager, hub
}

func dialTestConnection(
	t *testing.T,
	serverURL string,
	manager *auth.TokenManager,
	userID string,
	username string,
	role string,
) *websocket.Conn {
	t.Helper()

	rawToken, err := manager.IssueAccessToken(userID, username, role)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	connection, response, err := websocket.DefaultDialer.Dial(websocketURL(serverURL)+"?token="+rawToken, nil)
	if err != nil {
		if response != nil {
			t.Fatalf("dial websocket: %v, status=%d", err, response.StatusCode)
		}
		t.Fatalf("dial websocket: %v", err)
	}
	return connection
}

func readEvent(t *testing.T, connection *websocket.Conn) Event {
	t.Helper()
	if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	_, data, err := connection.ReadMessage()
	if err != nil {
		t.Fatalf("read event: %v", err)
	}

	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	return event
}

func websocketURL(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + "/ws"
}

var _ AccessRepository = fakeAccessRepository{}
