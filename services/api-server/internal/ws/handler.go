package ws

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"ptt-fleet/services/api-server/internal/apiutil"
	"ptt-fleet/services/api-server/internal/auth"
	"ptt-fleet/services/api-server/internal/gps"
	"ptt-fleet/services/api-server/internal/ptt"
	"ptt-fleet/services/api-server/internal/sos"
)

type GPSRecorder interface {
	Record(ctx context.Context, userID string, update gps.Update) (gps.Location, error)
}

type SOSService interface {
	Create(ctx context.Context, userID string, input sos.CreateInput) (sos.Event, error)
	Acknowledge(ctx context.Context, operatorID string, eventID string) (sos.Acknowledgement, error)
}

type Handler struct {
	tokens     *auth.TokenManager
	repository AccessRepository
	gps        GPSRecorder
	sos        SOSService
	ptt        *ptt.Manager
	hub        *Hub
	upgrader   websocket.Upgrader
}

func NewHandler(
	tokens *auth.TokenManager,
	repository AccessRepository,
	gpsRecorder GPSRecorder,
	sosService SOSService,
	pttManager *ptt.Manager,
	hub *Hub,
	allowedOrigins []string,
) *Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[strings.TrimSuffix(origin, "/")] = struct{}{}
	}

	return &Handler{
		tokens:     tokens,
		repository: repository,
		gps:        gpsRecorder,
		sos:        sosService,
		ptt:        pttManager,
		hub:        hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(request *http.Request) bool {
				origin := strings.TrimSuffix(request.Header.Get("Origin"), "/")
				if origin == "" {
					return true
				}
				_, ok := allowed[origin]
				return ok
			},
		},
	}
}

func (h *Handler) Connect(c *gin.Context) {
	rawToken := c.Query("token")
	if rawToken == "" {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "WebSocket token is required", nil)
		return
	}

	claims, err := h.tokens.ParseAccessToken(rawToken)
	if err != nil {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "WebSocket token is invalid or expired", nil)
		return
	}

	identity, err := h.repository.ActiveIdentity(c.Request.Context(), claims.Subject)
	if errors.Is(err, ErrUserNotFound) {
		apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "User no longer exists", nil)
		return
	}
	if errors.Is(err, ErrUserDisabled) {
		apiutil.Error(c, http.StatusForbidden, "user_disabled", "User account is disabled", nil)
		return
	}
	if err != nil {
		apiutil.Error(c, http.StatusServiceUnavailable, "server_error", "Unable to validate WebSocket user", nil)
		return
	}

	socket, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	connection, err := h.hub.NewConnection(identity, socket)
	if err != nil {
		_ = socket.Close()
		return
	}

	socket.SetReadLimit(maxMessageSize)
	connection.TouchHeartbeat()

	go connection.writeLoop()
	connection.Send(NewEvent("connection.ready", "", map[string]any{
		"connectionId": connection.ID,
		"userId":       connection.UserID,
		"role":         connection.Role,
	}))
	h.hub.Register(connection)
	defer func() {
		h.releaseConnectionPTT(connection)
		h.hub.Unregister(connection)
	}()

	h.readLoop(c, connection)
}

func (h *Handler) readLoop(c *gin.Context, connection *Connection) {
	for {
		messageType, data, err := connection.socket.ReadMessage()
		if err != nil {
			return
		}
		if messageType == websocket.BinaryMessage {
			h.handleAudioFrame(connection, data)
			continue
		}
		if messageType != websocket.TextMessage {
			connection.Send(NewErrorEvent("", "validation_error", "Unsupported WebSocket message type", nil))
			continue
		}

		event, err := ParseEvent(data)
		if err != nil {
			connection.Send(NewErrorEvent("", "validation_error", "Event must include type, RFC3339 timestamp, and object payload", nil))
			continue
		}

		switch event.Type {
		case "heartbeat":
			connection.TouchHeartbeat()
		case "group.join":
			h.handleGroupJoin(c, connection, event)
		case "gps.update":
			h.handleGPSUpdate(c, connection, event)
		case "sos.create":
			h.handleSOSCreate(c, connection, event)
		case "sos.ack":
			h.handleSOSAck(c, connection, event)
		case "ptt.start":
			h.handlePTTStart(c, connection, event)
		case "ptt.stop":
			h.handlePTTStop(c, connection, event)
		default:
			connection.Send(NewErrorEvent(event.RequestID, "validation_error", "Unsupported event type", map[string]any{
				"type": event.Type,
			}))
		}
	}
}

func (h *Handler) handlePTTStart(c *gin.Context, connection *Connection, event Event) {
	var payload struct {
		GroupID string `json:"groupId"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.GroupID == "" {
		connection.Send(NewErrorEvent(event.RequestID, "validation_error", "groupId is required", nil))
		return
	}
	if !connection.HasJoinedGroup(payload.GroupID) {
		connection.Send(NewErrorEvent(event.RequestID, "group_not_joined", "Join the group before starting PTT", nil))
		return
	}

	session, busy, err := h.ptt.Start(
		c.Request.Context(),
		payload.GroupID,
		connection.UserID,
		connection.ID,
	)
	if errors.Is(err, ptt.ErrBusy) {
		connection.Send(NewEvent("ptt.busy", event.RequestID, map[string]any{
			"groupId":       payload.GroupID,
			"speakerUserId": busy.SpeakerUserID,
		}))
		return
	}
	if err != nil {
		connection.Send(NewErrorEvent(event.RequestID, "server_error", "Unable to start PTT session", nil))
		return
	}

	connection.Send(NewEvent("ptt.granted", event.RequestID, map[string]any{
		"sessionId": session.ID,
		"groupId":   session.GroupID,
	}))
	h.hub.BroadcastToGroup(session.GroupID, NewEvent("ptt.started", "", map[string]any{
		"sessionId":     session.ID,
		"groupId":       session.GroupID,
		"speakerUserId": session.SpeakerUserID,
	}))
}

func (h *Handler) handlePTTStop(c *gin.Context, connection *Connection, event Event) {
	var payload struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.SessionID == "" {
		connection.Send(NewErrorEvent(event.RequestID, "validation_error", "sessionId is required", nil))
		return
	}

	session, err := h.ptt.Stop(
		c.Request.Context(),
		payload.SessionID,
		connection.UserID,
		connection.ID,
		"user_stop",
	)
	if errors.Is(err, ptt.ErrInvalidSession) || errors.Is(err, ptt.ErrNotSpeaker) {
		connection.Send(NewErrorEvent(event.RequestID, "forbidden", "PTT session is not owned by this connection", nil))
		return
	}
	if err != nil {
		connection.Send(NewErrorEvent(event.RequestID, "server_error", "Unable to stop PTT session", nil))
		return
	}

	h.broadcastPTTStopped(session, event.RequestID, "user_stop")
}

func (h *Handler) handleAudioFrame(connection *Connection, data []byte) {
	if len(data) <= 25 || data[0] != 0x01 {
		return
	}
	sessionID := binaryUUID(data[1:17])
	sequence := binary.BigEndian.Uint64(data[17:25])
	session, gap, err := h.ptt.ValidateFrame(sessionID, connection.UserID, connection.ID, sequence)
	if err != nil {
		return
	}
	if gap {
		log.Printf("PTT sequence gap session=%s sequence=%d", sessionID, sequence)
	}

	downlink := append([]byte(nil), data...)
	downlink[0] = 0x02
	h.hub.BroadcastBinaryToGroup(session.GroupID, connection.ID, downlink)
}

func (h *Handler) releaseConnectionPTT(connection *Connection) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, session := range h.ptt.ReleaseConnection(ctx, connection.ID) {
		h.broadcastPTTStopped(session, "", "disconnect")
	}
}

func (h *Handler) broadcastPTTStopped(session ptt.Session, requestID string, reason string) {
	h.hub.BroadcastToGroup(session.GroupID, NewEvent("ptt.stopped", requestID, map[string]any{
		"sessionId":     session.ID,
		"groupId":       session.GroupID,
		"speakerUserId": session.SpeakerUserID,
		"reason":        reason,
	}))
}

func binaryUUID(value []byte) string {
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	)
}

func (h *Handler) handleSOSCreate(c *gin.Context, connection *Connection, event Event) {
	var payload sos.CreateInput
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		connection.Send(NewErrorEvent(event.RequestID, "validation_error", "SOS payload is invalid", nil))
		return
	}

	created, err := h.sos.Create(c.Request.Context(), connection.UserID, payload)
	if errors.Is(err, sos.ErrInvalidEvent) {
		connection.Send(NewErrorEvent(event.RequestID, "validation_error", "SOS location or message is invalid", nil))
		return
	}
	if err != nil {
		connection.Send(NewErrorEvent(event.RequestID, "server_error", "Unable to create SOS event", nil))
		return
	}

	h.hub.BroadcastToOperators(NewEvent("sos.created", event.RequestID, created))
}

func (h *Handler) handleSOSAck(c *gin.Context, connection *Connection, event Event) {
	if !isOperatorRole(connection.Role) {
		connection.Send(NewErrorEvent(event.RequestID, "forbidden", "Only operators can acknowledge SOS events", nil))
		return
	}

	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.ID == "" {
		connection.Send(NewErrorEvent(event.RequestID, "validation_error", "SOS id is required", nil))
		return
	}

	acknowledgement, err := h.sos.Acknowledge(c.Request.Context(), connection.UserID, payload.ID)
	if errors.Is(err, sos.ErrNotOpen) {
		connection.Send(NewErrorEvent(event.RequestID, "conflict", "SOS event is no longer open", nil))
		return
	}
	if errors.Is(err, sos.ErrInvalidEvent) {
		connection.Send(NewErrorEvent(event.RequestID, "validation_error", "SOS id is invalid", nil))
		return
	}
	if err != nil {
		connection.Send(NewErrorEvent(event.RequestID, "server_error", "Unable to acknowledge SOS event", nil))
		return
	}

	h.hub.BroadcastToOperators(NewEvent("sos.acked", event.RequestID, acknowledgement))
}

func (h *Handler) handleGPSUpdate(c *gin.Context, connection *Connection, event Event) {
	var payload struct {
		Lat      *float64 `json:"lat"`
		Lng      *float64 `json:"lng"`
		Speed    *float64 `json:"speed"`
		Heading  *float64 `json:"heading"`
		Accuracy *float64 `json:"accuracy"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.Lat == nil || payload.Lng == nil {
		connection.Send(NewErrorEvent(event.RequestID, "validation_error", "lat and lng are required", nil))
		return
	}

	location, err := h.gps.Record(c.Request.Context(), connection.UserID, gps.Update{
		Lat:      *payload.Lat,
		Lng:      *payload.Lng,
		Speed:    payload.Speed,
		Heading:  payload.Heading,
		Accuracy: payload.Accuracy,
	})
	if errors.Is(err, gps.ErrInvalidLocation) {
		connection.Send(NewErrorEvent(event.RequestID, "validation_error", "GPS coordinates or measurements are invalid", nil))
		return
	}
	if err != nil {
		connection.Send(NewErrorEvent(event.RequestID, "server_error", "Unable to save GPS location", nil))
		return
	}

	h.hub.BroadcastToOperators(NewEvent("gps.updated", event.RequestID, location))
}

func (h *Handler) handleGroupJoin(c *gin.Context, connection *Connection, event Event) {
	var payload struct {
		GroupID string `json:"groupId"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.GroupID == "" {
		connection.Send(NewErrorEvent(event.RequestID, "validation_error", "groupId is required", nil))
		return
	}

	err := h.repository.CanJoinGroup(c.Request.Context(), connection.UserID, payload.GroupID)
	if errors.Is(err, ErrGroupNotAllowed) {
		connection.Send(NewErrorEvent(event.RequestID, "forbidden", "User is not a member of this group", nil))
		return
	}
	if err != nil {
		connection.Send(NewErrorEvent(event.RequestID, "server_error", "Unable to validate group membership", nil))
		return
	}

	connection.JoinGroup(payload.GroupID)
	connection.Send(NewEvent("group.joined", event.RequestID, map[string]any{
		"groupId": payload.GroupID,
	}))
}

func HeartbeatWindow() time.Duration {
	return heartbeatWindow
}
