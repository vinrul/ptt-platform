package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeTimeout    = 10 * time.Second
	heartbeatWindow = 90 * time.Second
	maxMessageSize  = 64 * 1024
)

type Connection struct {
	ID            string
	UserID        string
	Username      string
	Role          string
	JoinedGroups  map[string]struct{}
	LastHeartbeat time.Time

	socket    *websocket.Conn
	send      chan OutboundEvent
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
}

func newConnection(id string, identity Identity, socket *websocket.Conn) *Connection {
	now := time.Now().UTC()
	return &Connection{
		ID:            id,
		UserID:        identity.UserID,
		Username:      identity.Username,
		Role:          identity.Role,
		JoinedGroups:  make(map[string]struct{}),
		LastHeartbeat: now,
		socket:        socket,
		send:          make(chan OutboundEvent, 64),
		done:          make(chan struct{}),
	}
}

func (c *Connection) Send(event OutboundEvent) bool {
	select {
	case <-c.done:
		return false
	case c.send <- event:
		return true
	default:
		c.Close(websocket.ClosePolicyViolation, "client is too slow")
		return false
	}
}

func (c *Connection) TouchHeartbeat() {
	now := time.Now().UTC()
	c.mu.Lock()
	c.LastHeartbeat = now
	c.mu.Unlock()
	if c.socket != nil {
		_ = c.socket.SetReadDeadline(now.Add(heartbeatWindow))
	}
}

func (c *Connection) JoinGroup(groupID string) {
	c.mu.Lock()
	c.JoinedGroups[groupID] = struct{}{}
	c.mu.Unlock()
}

func (c *Connection) HasJoinedGroup(groupID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.JoinedGroups[groupID]
	return exists
}

func (c *Connection) Close(code int, reason string) {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.socket == nil {
			return
		}
		deadline := time.Now().Add(writeTimeout)
		_ = c.socket.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), deadline)
		_ = c.socket.Close()
	})
}

func (c *Connection) writeLoop() {
	for {
		select {
		case event := <-c.send:
			_ = c.socket.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.socket.WriteJSON(event); err != nil {
				c.Close(websocket.CloseGoingAway, "write failed")
				return
			}
		case <-c.done:
			return
		}
	}
}

func isOperatorRole(role string) bool {
	return role == "super_admin" || role == "dispatcher" || role == "supervisor"
}
