package ptt

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrBusy           = errors.New("PTT group is busy")
	ErrInvalidSession = errors.New("invalid PTT session")
	ErrNotSpeaker     = errors.New("connection does not own PTT session")
)

type Session struct {
	ID                string
	GroupID           string
	SpeakerUserID     string
	TargetUserID      string
	OwnerConnectionID string
	StartedAt         time.Time
	LastSequence      uint64
	HasSequence       bool
}

type Repository interface {
	CreateSession(ctx context.Context, groupID string, speakerUserID string) (Session, error)
	StopSession(ctx context.Context, sessionID string, reason string, startedAt time.Time) error
}

type Manager struct {
	mu      sync.Mutex
	repo    Repository
	byGroup map[string]*Session
	byID    map[string]*Session
}

func NewManager(repository Repository) *Manager {
	return &Manager{
		repo:    repository,
		byGroup: make(map[string]*Session),
		byID:    make(map[string]*Session),
	}
}

func (m *Manager) Start(
	ctx context.Context,
	groupID string,
	speakerUserID string,
	connectionID string,
	targetUserID string,
) (Session, *Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if active := m.byGroup[groupID]; active != nil {
		copy := *active
		return Session{}, &copy, ErrBusy
	}

	session, err := m.repo.CreateSession(ctx, groupID, speakerUserID)
	if err != nil {
		return Session{}, nil, err
	}
	session.OwnerConnectionID = connectionID
	session.TargetUserID = targetUserID
	stored := session
	m.byGroup[groupID] = &stored
	m.byID[session.ID] = &stored
	return session, nil, nil
}

func (m *Manager) Stop(
	ctx context.Context,
	sessionID string,
	userID string,
	connectionID string,
	reason string,
) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := m.byID[sessionID]
	if session == nil {
		return Session{}, ErrInvalidSession
	}
	if session.SpeakerUserID != userID || session.OwnerConnectionID != connectionID {
		return Session{}, ErrNotSpeaker
	}
	if err := m.repo.StopSession(ctx, session.ID, reason, session.StartedAt); err != nil {
		return Session{}, err
	}

	copy := *session
	delete(m.byID, session.ID)
	delete(m.byGroup, session.GroupID)
	return copy, nil
}

func (m *Manager) ReleaseConnection(ctx context.Context, connectionID string) []Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	released := make([]Session, 0)
	for sessionID, session := range m.byID {
		if session.OwnerConnectionID != connectionID {
			continue
		}
		_ = m.repo.StopSession(ctx, session.ID, "disconnect", session.StartedAt)
		released = append(released, *session)
		delete(m.byID, sessionID)
		delete(m.byGroup, session.GroupID)
	}
	return released
}

func (m *Manager) ValidateFrame(
	sessionID string,
	userID string,
	connectionID string,
	sequence uint64,
) (Session, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := m.byID[sessionID]
	if session == nil {
		return Session{}, false, ErrInvalidSession
	}
	if session.SpeakerUserID != userID || session.OwnerConnectionID != connectionID {
		return Session{}, false, ErrNotSpeaker
	}

	gap := session.HasSequence && sequence != session.LastSequence+1
	session.LastSequence = sequence
	session.HasSequence = true
	return *session, gap, nil
}
