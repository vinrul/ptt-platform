package ws

import (
	"regexp"
	"testing"
)

func TestHubTracksPresenceAcrossMultipleConnections(t *testing.T) {
	hub := NewHub()
	identity := Identity{UserID: "user-1", Username: "field1", Role: "field_user"}

	first := newConnection("connection-1", identity, nil)
	second := newConnection("connection-2", identity, nil)

	hub.Register(first)
	hub.Register(second)

	if count := hub.ConnectionCount(); count != 2 {
		t.Fatalf("expected 2 connections, got %d", count)
	}
	if count := hub.UserConnectionCount(identity.UserID); count != 2 {
		t.Fatalf("expected 2 user connections, got %d", count)
	}

	hub.Unregister(first)
	if count := hub.UserConnectionCount(identity.UserID); count != 1 {
		t.Fatalf("expected user to remain online with 1 connection, got %d", count)
	}

	hub.Unregister(second)
	if count := hub.UserConnectionCount(identity.UserID); count != 0 {
		t.Fatalf("expected user to be offline, got %d connections", count)
	}
}

func TestConnectionTracksJoinedGroups(t *testing.T) {
	connection := newConnection(
		"connection-1",
		Identity{UserID: "user-1", Username: "field1", Role: "field_user"},
		nil,
	)

	connection.JoinGroup("group-1")
	if !connection.HasJoinedGroup("group-1") {
		t.Fatal("expected connection to track joined group")
	}
}

func TestHubFindsUserJoinedToGroup(t *testing.T) {
	hub := NewHub()
	connection := newConnection(
		"connection-1",
		Identity{UserID: "user-1", Username: "field1", Role: "field_user"},
		nil,
	)
	connection.JoinGroup("group-1")
	hub.Register(connection)
	defer hub.Unregister(connection)

	if !hub.UserHasJoinedGroup("user-1", "group-1") {
		t.Fatal("expected online user to be found in joined group")
	}
	if hub.UserHasJoinedGroup("user-2", "group-1") {
		t.Fatal("did not expect unrelated user in joined group")
	}
}

func TestHeartbeatWindowIsNinetySeconds(t *testing.T) {
	if HeartbeatWindow().Seconds() != 90 {
		t.Fatalf("expected 90 second heartbeat window, got %s", HeartbeatWindow())
	}
}

func TestConnectionIDIsUUIDV4(t *testing.T) {
	connectionID, err := newConnectionID()
	if err != nil {
		t.Fatalf("create connection id: %v", err)
	}

	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(connectionID) {
		t.Fatalf("expected UUID v4 connection id, got %s", connectionID)
	}
}
