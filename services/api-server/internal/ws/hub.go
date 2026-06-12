package ws

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu          sync.RWMutex
	connections map[string]*Connection
	userCounts  map[string]int
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]*Connection),
		userCounts:  make(map[string]int),
	}
}

func (h *Hub) NewConnection(identity Identity, socket *websocket.Conn) (*Connection, error) {
	connectionID, err := newConnectionID()
	if err != nil {
		return nil, err
	}
	return newConnection(connectionID, identity, socket), nil
}

func (h *Hub) Register(connection *Connection) {
	h.mu.Lock()
	h.connections[connection.ID] = connection
	h.userCounts[connection.UserID]++
	firstConnection := h.userCounts[connection.UserID] == 1
	h.mu.Unlock()

	if firstConnection {
		h.broadcastPresence(connection.UserID, "online")
	}
}

func (h *Hub) Unregister(connection *Connection) {
	joinedGroupIDs := connection.JoinedGroupIDs()
	h.mu.Lock()
	if _, exists := h.connections[connection.ID]; !exists {
		h.mu.Unlock()
		return
	}

	delete(h.connections, connection.ID)
	h.userCounts[connection.UserID]--
	lastConnection := h.userCounts[connection.UserID] == 0
	if lastConnection {
		delete(h.userCounts, connection.UserID)
	}
	h.mu.Unlock()

	connection.Close(websocket.CloseNormalClosure, "connection closed")
	if lastConnection {
		h.broadcastPresence(connection.UserID, "offline")
	}
	for _, groupID := range joinedGroupIDs {
		if !h.UserHasJoinedGroup(connection.UserID, groupID) {
			h.BroadcastPresenceToGroup(groupID, connection.UserID, "offline")
		}
	}
}

func (h *Hub) CloseAll() {
	h.mu.RLock()
	connections := make([]*Connection, 0, len(h.connections))
	for _, connection := range h.connections {
		connections = append(connections, connection)
	}
	h.mu.RUnlock()

	for _, connection := range connections {
		h.Unregister(connection)
	}
}

func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

func (h *Hub) UserConnectionCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.userCounts[userID]
}

func (h *Hub) ConnectionByID(connectionID string) *Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connections[connectionID]
}

func (h *Hub) BroadcastToOperators(event OutboundEvent) {
	h.mu.RLock()
	recipients := make([]*Connection, 0)
	for _, connection := range h.connections {
		if isOperatorRole(connection.Role) {
			recipients = append(recipients, connection)
		}
	}
	h.mu.RUnlock()

	for _, connection := range recipients {
		connection.Send(event)
	}
}

func (h *Hub) BroadcastToGroup(groupID string, event OutboundEvent) {
	h.mu.RLock()
	recipients := make([]*Connection, 0)
	for _, connection := range h.connections {
		if connection.HasJoinedGroup(groupID) {
			recipients = append(recipients, connection)
		}
	}
	h.mu.RUnlock()

	for _, connection := range recipients {
		connection.Send(event)
	}
}

func (h *Hub) SendToUser(userID string, event OutboundEvent) int {
	h.mu.RLock()
	recipients := make([]*Connection, 0)
	for _, connection := range h.connections {
		if connection.UserID == userID {
			recipients = append(recipients, connection)
		}
	}
	h.mu.RUnlock()

	sent := 0
	for _, connection := range recipients {
		if connection.Send(event) {
			sent++
		}
	}
	return sent
}

func (h *Hub) BroadcastBinaryToGroup(groupID string, senderConnectionID string, data []byte) {
	h.mu.RLock()
	recipients := make([]*Connection, 0)
	for _, connection := range h.connections {
		if connection.ID != senderConnectionID && connection.HasJoinedGroup(groupID) {
			recipients = append(recipients, connection)
		}
	}
	h.mu.RUnlock()

	for _, connection := range recipients {
		connection.SendBinary(data)
	}
}

func (h *Hub) BroadcastBinaryToUserInGroup(
	groupID string,
	userID string,
	senderConnectionID string,
	data []byte,
) {
	h.mu.RLock()
	recipients := make([]*Connection, 0)
	for _, connection := range h.connections {
		if connection.ID != senderConnectionID &&
			connection.UserID == userID &&
			connection.HasJoinedGroup(groupID) {
			recipients = append(recipients, connection)
		}
	}
	h.mu.RUnlock()

	for _, connection := range recipients {
		connection.SendBinary(data)
	}
}

func (h *Hub) UserHasJoinedGroup(userID string, groupID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, connection := range h.connections {
		if connection.UserID == userID && connection.HasJoinedGroup(groupID) {
			return true
		}
	}
	return false
}

func (h *Hub) OnlineUserIDsInGroup(groupID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := make(map[string]struct{})
	userIDs := make([]string, 0)
	for _, connection := range h.connections {
		if !connection.HasJoinedGroup(groupID) {
			continue
		}
		if _, exists := seen[connection.UserID]; exists {
			continue
		}
		seen[connection.UserID] = struct{}{}
		userIDs = append(userIDs, connection.UserID)
	}
	return userIDs
}

func (h *Hub) BroadcastPresenceToGroup(groupID string, userID string, status string) {
	h.BroadcastToGroup(groupID, NewEvent("presence.updated", "", map[string]any{
		"userId":     userID,
		"status":     status,
		"lastSeenAt": time.Now().UTC().Format(time.RFC3339),
	}))
}

func (h *Hub) BroadcastDirectPTT(
	groupID string,
	speakerUserID string,
	targetUserID string,
	event OutboundEvent,
) {
	h.mu.RLock()
	recipients := make([]*Connection, 0)
	for _, connection := range h.connections {
		if (connection.UserID == speakerUserID || connection.UserID == targetUserID) &&
			connection.HasJoinedGroup(groupID) {
			recipients = append(recipients, connection)
		}
	}
	h.mu.RUnlock()

	for _, connection := range recipients {
		connection.Send(event)
	}
}

func (h *Hub) broadcastPresence(userID string, status string) {
	event := NewEvent("presence.updated", "", map[string]any{
		"userId":     userID,
		"status":     status,
		"lastSeenAt": time.Now().UTC().Format(time.RFC3339),
	})

	h.BroadcastToOperators(event)
}

func newConnectionID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	), nil
}
