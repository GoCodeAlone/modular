package auth

import (
	"context"
	"sync"
	"time"
)

// MemoryUserStore implements UserStore interface using in-memory storage
type MemoryUserStore struct {
	users map[string]*User
	mutex sync.RWMutex
}

// NewMemoryUserStore creates a new in-memory user store
func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		users: make(map[string]*User),
	}
}

// GetUser retrieves a user by ID
func (s *MemoryUserStore) GetUser(ctx context.Context, userID string) (*User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	user, exists := s.users[userID]
	if !exists {
		return nil, ErrUserNotFound
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (s *MemoryUserStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}

	return nil, ErrUserNotFound
}

// CreateUser creates a new user
func (s *MemoryUserStore) CreateUser(ctx context.Context, user *User) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if user already exists
	if _, exists := s.users[user.ID]; exists {
		return ErrUserAlreadyExists
	}

	// Check if email already exists
	for _, existingUser := range s.users {
		if existingUser.Email == user.Email {
			return ErrUserAlreadyExists
		}
	}

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	s.users[user.ID] = user

	return nil
}

// UpdateUser updates an existing user
func (s *MemoryUserStore) UpdateUser(ctx context.Context, user *User) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.users[user.ID]; !exists {
		return ErrUserNotFound
	}

	user.UpdatedAt = time.Now()
	s.users[user.ID] = user

	return nil
}

// DeleteUser deletes a user
func (s *MemoryUserStore) DeleteUser(ctx context.Context, userID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.users[userID]; !exists {
		return ErrUserNotFound
	}

	delete(s.users, userID)
	return nil
}

// MemorySessionStore implements SessionStore interface using in-memory storage
type MemorySessionStore struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
}

// NewMemorySessionStore creates a new in-memory session store
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*Session),
	}
}

// Store stores a session
func (s *MemorySessionStore) Store(ctx context.Context, session *Session) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.sessions[session.ID] = session
	return nil
}

// Get retrieves a session by ID
func (s *MemorySessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	return session, nil
}

// Delete removes a session
func (s *MemorySessionStore) Delete(ctx context.Context, sessionID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.sessions, sessionID)
	return nil
}

// Cleanup removes expired sessions
func (s *MemorySessionStore) Cleanup(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) || !session.Active {
			delete(s.sessions, id)
		}
	}

	return nil
}
