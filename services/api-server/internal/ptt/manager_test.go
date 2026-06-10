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
