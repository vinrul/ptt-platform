package ptt

import (
	"context"
	"testing"
	"time"
)

type fakeRepository struct {
	nextID  int
	stopped []string
}

func (r *fakeRepository) CreateSession(_ context.Context, groupID string, speakerUserID string) (Session, error) {
	r.nextID++
	return Session{
		ID:            "session-1",
		GroupID:       groupID,
		SpeakerUserID: speakerUserID,
		StartedAt:     time.Now(),
	}, nil
}

func (r *fakeRepository) StopSession(_ context.Context, sessionID string, reason string, _ time.Time) error {
	r.stopped = append(r.stopped, sessionID+":"+reason)
	return nil
}

func TestManagerEnforcesGroupLockAndSequence(t *testing.T) {
	repository := &fakeRepository{}
	manager := NewManager(repository)

	session, _, err := manager.Start(context.Background(), "group-1", "user-1", "connection-1", "")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	_, busy, err := manager.Start(context.Background(), "group-1", "user-2", "connection-2", "")
	if err != ErrBusy || busy == nil || busy.SpeakerUserID != "user-1" {
		t.Fatalf("expected busy session, got busy=%#v err=%v", busy, err)
	}

	if _, gap, err := manager.ValidateFrame(session.ID, "user-1", "connection-1", 1); err != nil || gap {
		t.Fatalf("expected first frame without gap, gap=%v err=%v", gap, err)
	}
	if _, gap, err := manager.ValidateFrame(session.ID, "user-1", "connection-1", 3); err != nil || !gap {
		t.Fatalf("expected sequence gap, gap=%v err=%v", gap, err)
	}

	stopped, err := manager.Stop(context.Background(), session.ID, "user-1", "connection-1", "user_stop")
	if err != nil || stopped.ID != session.ID {
		t.Fatalf("stop session: %#v %v", stopped, err)
	}
}

func TestManagerReleasesSessionOnDisconnect(t *testing.T) {
	repository := &fakeRepository{}
	manager := NewManager(repository)
	_, _, err := manager.Start(context.Background(), "group-1", "user-1", "connection-1", "")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	released := manager.ReleaseConnection(context.Background(), "connection-1")
	if len(released) != 1 || released[0].GroupID != "group-1" {
		t.Fatalf("expected released session, got %#v", released)
	}
	if len(repository.stopped) != 1 || repository.stopped[0] != "session-1:disconnect" {
		t.Fatalf("expected disconnect persistence, got %#v", repository.stopped)
	}
}

func TestManagerQueuesAndPromotesRequestsInOrder(t *testing.T) {
	repository := &fakeRepository{}
	manager := NewManager(repository)
	_, _, err := manager.Start(context.Background(), "group-1", "user-1", "connection-1", "")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	position, err := manager.Enqueue(QueuedRequest{
		GroupID:           "group-1",
		SpeakerUserID:     "user-2",
		OwnerConnectionID: "connection-2",
		RequestID:         "request-2",
	})
	if err != nil || position != 1 {
		t.Fatalf("enqueue first request: position=%d err=%v", position, err)
	}
	position, err = manager.Enqueue(QueuedRequest{
		GroupID:           "group-1",
		SpeakerUserID:     "user-3",
		OwnerConnectionID: "connection-3",
		RequestID:         "request-3",
	})
	if err != nil || position != 2 {
		t.Fatalf("enqueue second request: position=%d err=%v", position, err)
	}

	active := manager.byGroup["group-1"]
	if _, err := manager.Stop(context.Background(), active.ID, "user-1", "connection-1", "user_stop"); err != nil {
		t.Fatalf("stop active session: %v", err)
	}
	session, request, err := manager.StartNext(context.Background(), "group-1")
	if err != nil || request == nil {
		t.Fatalf("promote next request: request=%#v err=%v", request, err)
	}
	if session.SpeakerUserID != "user-2" || request.RequestID != "request-2" {
		t.Fatalf("expected user-2 first, got session=%#v request=%#v", session, request)
	}
}

func TestManagerCancelsQueuedRequest(t *testing.T) {
	manager := NewManager(&fakeRepository{})
	_, _ = manager.Enqueue(QueuedRequest{
		GroupID:           "group-1",
		SpeakerUserID:     "user-2",
		OwnerConnectionID: "connection-2",
	})

	if !manager.CancelQueued("group-1", "connection-2") {
		t.Fatal("expected queued request to be cancelled")
	}
	session, request, err := manager.StartNext(context.Background(), "group-1")
	if err != nil || request != nil || session.ID != "" {
		t.Fatalf("expected empty queue, got session=%#v request=%#v err=%v", session, request, err)
	}
}
