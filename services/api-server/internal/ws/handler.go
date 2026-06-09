package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"ptt-fleet/services/api-server/internal/apiutil"
	"ptt-fleet/services/api-server/internal/auth"
	"ptt-fleet/services/api-server/internal/gps"
)

type GPSRecorder interface {
	Record(ctx context.Context, userID string, update gps.Update) (gps.Location, error)
}

type Handler struct {
	tokens     *auth.TokenManager
	repository AccessRepository
	gps        GPSRecorder
	hub        *Hub
	upgrader   websocket.Upgrader
}

func NewHandler(tokens *auth.TokenManager, repository AccessRepository, gpsRecorder GPSRecorder, hub *Hub) *Handler {
	return &Handler{
		tokens:     tokens,
		repository: repository,
		gps:        gpsRecorder,
		hub:        hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(_ *http.Request) bool {
				return true
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
	defer h.hub.Unregister(connection)

	h.readLoop(c, connection)
}

func (h *Handler) readLoop(c *gin.Context, connection *Connection) {
	for {
		messageType, data, err := connection.socket.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			connection.Send(NewErrorEvent("", "validation_error", "Only JSON text events are accepted in this phase", nil))
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
		default:
			connection.Send(NewErrorEvent(event.RequestID, "validation_error", "Unsupported event type", map[string]any{
				"type": event.Type,
			}))
		}
	}
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
