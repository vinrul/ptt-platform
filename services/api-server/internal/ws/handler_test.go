package ws

import (
	"context"
	"encoding/binary"
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
	"ptt-fleet/services/api-server/internal/ptt"
	"ptt-fleet/services/api-server/internal/sos"
)

type fakeAccessRepository struct {
	identity   Identity
	identities map[string]Identity
	joinErr    error
}

type fakeGPSRecorder struct {
	location gps.Location
	err      error
}

type fakeSOSService struct{}

type fakePTTRepository struct{}

func (fakePTTRepository) CreateSession(_ context.Context, groupID string, speakerUserID string) (ptt.Session, error) {
	return ptt.Session{
		ID:            "11111111-1111-4111-8111-111111111111",
		GroupID:       groupID,
		SpeakerUserID: speakerUserID,
		StartedAt:     time.Now(),
	}, nil
}

func (fakePTTRepository) StopSession(_ context.Context, _ string, _ string, _ time.Time) error {
	return nil
}

func (fakeSOSService) Create(_ context.Context, userID string, input sos.CreateInput) (sos.Event, error) {
	return sos.Event{
		ID:      "sos-1",
		UserID:  userID,
		Lat:     input.Lat,
		Lng:     input.Lng,
		Message: input.Message,
		Status:  "open",
	}, nil
}

func (fakeSOSService) Acknowledge(_ context.Context, operatorID string, eventID string) (sos.Acknowledgement, error) {
	return sos.Acknowledgement{
		ID:             eventID,
		Status:         "ack",
		AcknowledgedBy: operatorID,
	}, nil
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
	if identity, exists := r.identities[userID]; exists {
		return identity, nil
	}
	if r.identity.UserID != userID {
		return Identity{}, ErrUserNotFound
	}
	return r.identity, nil
}

func TestHandlerRelaysPTTAudioWithinJoinedGroup(t *testing.T) {
	server, manager, hub := newWebSocketTestServer(t, fakeAccessRepository{
		identities: map[string]Identity{
			"user-1": {UserID: "user-1", Username: "field1", Role: "field_user"},
			"user-2": {UserID: "user-2", Username: "field2", Role: "field_user"},
		},
	})
	defer server.Close()
	defer hub.CloseAll()

	speaker := dialTestConnection(t, server.URL, manager, "user-1", "field1", "field_user")
	defer speaker.Close()
	listener := dialTestConnection(t, server.URL, manager, "user-2", "field2", "field_user")
	defer listener.Close()
	_ = readEvent(t, speaker)
	_ = readEvent(t, listener)

	for _, connection := range []*websocket.Conn{speaker, listener} {
		if err := connection.WriteJSON(NewEvent("group.join", "", map[string]any{"groupId": "group-1"})); err != nil {
			t.Fatalf("write group.join: %v", err)
		}
		if event := readEvent(t, connection); event.Type != "group.joined" {
			t.Fatalf("expected group.joined, got %s", event.Type)
		}
	}

	if err := speaker.WriteJSON(NewEvent("ptt.start", "req-start", map[string]any{"groupId": "group-1"})); err != nil {
		t.Fatalf("write ptt.start: %v", err)
	}
	granted := readEvent(t, speaker)
	if granted.Type != "ptt.granted" {
		t.Fatalf("expected ptt.granted, got %s", granted.Type)
	}
	startedSpeaker := readEvent(t, speaker)
	if startedSpeaker.Type != "ptt.started" {
		t.Fatalf("expected ptt.started for speaker, got %s", startedSpeaker.Type)
	}
	startedListener := readEvent(t, listener)
	if startedListener.Type != "ptt.started" {
		t.Fatalf("expected ptt.started for listener, got %s", startedListener.Type)
	}

	frame := make([]byte, 28)
	frame[0] = 0x01
	copy(frame[1:17], []byte{
		0x11, 0x11, 0x11, 0x11,
		0x11, 0x11,
		0x41, 0x11,
		0x81, 0x11,
		0x11, 0x11, 0x11, 0x11, 0x11, 0x11,
	})
	binary.BigEndian.PutUint64(frame[17:25], 1)
	copy(frame[25:], []byte{0x01, 0x02, 0x03})
	if err := speaker.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		t.Fatalf("write audio frame: %v", err)
	}

	if err := listener.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set listener deadline: %v", err)
	}
	messageType, downlink, err := listener.ReadMessage()
	if err != nil {
		t.Fatalf("read downlink: %v", err)
	}
	if messageType != websocket.BinaryMessage || downlink[0] != 0x02 {
		t.Fatalf("expected binary downlink, type=%d frame=%x", messageType, downlink)
	}
	if binary.BigEndian.Uint64(downlink[17:25]) != 1 {
		t.Fatalf("expected sequence 1, got %d", binary.BigEndian.Uint64(downlink[17:25]))
	}
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

func TestHandlerRejectsUnknownOrigin(t *testing.T) {
	server, manager, _ := newWebSocketTestServer(t, fakeAccessRepository{
		identity: Identity{UserID: "user-1", Username: "field1", Role: "field_user"},
	})
	defer server.Close()

	rawToken, err := manager.IssueAccessToken("user-1", "field1", "field_user")
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	headers := http.Header{}
	headers.Set("Origin", "https://evil.example")
	_, response, err := websocket.DefaultDialer.Dial(
		websocketURL(server.URL)+"?token="+rawToken,
		headers,
	)
	if err == nil {
		t.Fatal("expected unknown origin dial to fail")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403, got %#v", response)
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

func TestHandlerRejectsSOSAckFromFieldUser(t *testing.T) {
	server, manager, hub := newWebSocketTestServer(t, fakeAccessRepository{
		identity: Identity{UserID: "user-1", Username: "field1", Role: "field_user"},
	})
	defer server.Close()
	defer hub.CloseAll()

	connection := dialTestConnection(t, server.URL, manager, "user-1", "field1", "field_user")
	defer connection.Close()
	_ = readEvent(t, connection)

	if err := connection.WriteJSON(NewEvent("sos.ack", "req-ack", map[string]any{"id": "sos-1"})); err != nil {
		t.Fatalf("write sos.ack: %v", err)
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

func newWebSocketTestServer(t *testing.T, repository AccessRepository) (*httptest.Server, *auth.TokenManager, *Hub) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	manager := auth.NewTokenManager("12345678901234567890123456789012", time.Minute)
	hub := NewHub()
	handler := NewHandler(manager, repository, fakeGPSRecorder{
		location: gps.Location{RecordedAt: time.Now().UTC().Format(time.RFC3339)},
	}, fakeSOSService{}, ptt.NewManager(fakePTTRepository{}), hub, []string{"http://localhost:5173"})
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
