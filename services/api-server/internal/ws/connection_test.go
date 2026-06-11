package ws

import "testing"

func TestJoinGroupReplacesPreviousGroup(t *testing.T) {
	connection := newConnection("connection-1", Identity{
		UserID:   "user-1",
		Username: "field-1",
		Role:     "field_user",
	}, nil)

	connection.JoinGroup("group-1")
	connection.JoinGroup("group-2")

	if connection.HasJoinedGroup("group-1") {
		t.Fatal("expected previous group membership to be removed")
	}
	if !connection.HasJoinedGroup("group-2") {
		t.Fatal("expected current group membership")
	}
}
