package session

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type SessionDescriptor struct {
	Id        string
	StartedAt time.Time
	ExpiredAt time.Time
}

type SessionManager interface {
	// Create a session from scratch, it is the responsibility of the
	// concrete implementation to keep track of and enfore the session config (like expiration and metadata)
	CreateSession(ctx context.Context) (*SessionDescriptor, error)

	// If the session is exist and still valid, returns true, otherwise returns false.
	ValidateSession(ctx context.Context, sessionId string) bool
}

// `InMemorySessionManager` is an implementation of  `SessionManager` that keep track of session data in memory
type InMemorySessionManager struct {
	store sync.Map
	ttl   time.Duration
}

func NewInMemorySessionManager(ttl time.Duration) *InMemorySessionManager {
	return &InMemorySessionManager{
		ttl: ttl,
	}
}

func (m *InMemorySessionManager) CreateSession(_ context.Context) (*SessionDescriptor, error) {
	now := time.Now()
	descriptor := &SessionDescriptor{
		Id:        uuid.New().String(),
		StartedAt: now,
		ExpiredAt: now.Add(m.ttl),
	}
	m.store.Store(descriptor.Id, descriptor)
	return descriptor, nil
}

func (m *InMemorySessionManager) ValidateSession(_ context.Context, sessionId string) bool {
	val, ok := m.store.Load(sessionId)
	if !ok {
		return false
	}
	descriptor, ok := val.(*SessionDescriptor)
	if !ok {
		return false
	}
	return time.Now().Before(descriptor.ExpiredAt)
}
