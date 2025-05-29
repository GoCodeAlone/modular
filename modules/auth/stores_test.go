package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryUserStore(t *testing.T) {
	store := NewMemoryUserStore()
	ctx := context.Background()

	user := &User{
		ID:           "test-user-123",
		Email:        "test@example.com",
		PasswordHash: "hashed-password",
		Roles:        []string{"user"},
		Permissions:  []string{"read"},
		Active:       true,
		Metadata:     map[string]interface{}{"key": "value"},
	}

	// Test CreateUser
	err := store.CreateUser(ctx, user)
	require.NoError(t, err)
	assert.True(t, !user.CreatedAt.IsZero())
	assert.True(t, !user.UpdatedAt.IsZero())

	// Test duplicate user creation
	duplicateUser := &User{
		ID:    "test-user-123",
		Email: "different@example.com",
	}
	err = store.CreateUser(ctx, duplicateUser)
	assert.ErrorIs(t, err, ErrUserAlreadyExists)

	// Test duplicate email
	duplicateEmailUser := &User{
		ID:    "different-user",
		Email: "test@example.com",
	}
	err = store.CreateUser(ctx, duplicateEmailUser)
	assert.ErrorIs(t, err, ErrUserAlreadyExists)

	// Test GetUser
	retrievedUser, err := store.GetUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, retrievedUser.ID)
	assert.Equal(t, user.Email, retrievedUser.Email)
	assert.Equal(t, user.Roles, retrievedUser.Roles)
	assert.Equal(t, user.Permissions, retrievedUser.Permissions)
	assert.Equal(t, user.Active, retrievedUser.Active)
	assert.Equal(t, user.Metadata, retrievedUser.Metadata)

	// Test GetUserByEmail
	retrievedByEmail, err := store.GetUserByEmail(ctx, user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, retrievedByEmail.ID)

	// Test UpdateUser
	user.Email = "updated@example.com"
	user.Roles = []string{"user", "admin"}
	originalUpdatedAt := user.UpdatedAt
	time.Sleep(1 * time.Millisecond) // Ensure time difference

	err = store.UpdateUser(ctx, user)
	require.NoError(t, err)
	assert.True(t, user.UpdatedAt.After(originalUpdatedAt))

	updatedUser, err := store.GetUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated@example.com", updatedUser.Email)
	assert.Equal(t, []string{"user", "admin"}, updatedUser.Roles)

	// Test update non-existent user
	nonExistentUser := &User{ID: "non-existent"}
	err = store.UpdateUser(ctx, nonExistentUser)
	assert.ErrorIs(t, err, ErrUserNotFound)

	// Test DeleteUser
	err = store.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Verify user is deleted
	_, err = store.GetUser(ctx, user.ID)
	assert.ErrorIs(t, err, ErrUserNotFound)

	// Test delete non-existent user
	err = store.DeleteUser(ctx, "non-existent")
	assert.ErrorIs(t, err, ErrUserNotFound)

	// Test get non-existent user by email
	_, err = store.GetUserByEmail(ctx, "nonexistent@example.com")
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestMemorySessionStore(t *testing.T) {
	store := NewMemorySessionStore()
	ctx := context.Background()

	session := &Session{
		ID:        "test-session-123",
		UserID:    "test-user-123",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		IPAddress: "127.0.0.1",
		UserAgent: "test-browser",
		Active:    true,
		Metadata:  map[string]interface{}{"key": "value"},
	}

	// Test Store
	err := store.Store(ctx, session)
	require.NoError(t, err)

	// Test Get
	retrievedSession, err := store.Get(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrievedSession.ID)
	assert.Equal(t, session.UserID, retrievedSession.UserID)
	assert.Equal(t, session.IPAddress, retrievedSession.IPAddress)
	assert.Equal(t, session.UserAgent, retrievedSession.UserAgent)
	assert.Equal(t, session.Active, retrievedSession.Active)
	assert.Equal(t, session.Metadata, retrievedSession.Metadata)

	// Test update session (store again)
	session.Active = false
	err = store.Store(ctx, session)
	require.NoError(t, err)

	updatedSession, err := store.Get(ctx, session.ID)
	require.NoError(t, err)
	assert.False(t, updatedSession.Active)

	// Test Delete
	err = store.Delete(ctx, session.ID)
	require.NoError(t, err)

	// Verify session is deleted
	_, err = store.Get(ctx, session.ID)
	assert.ErrorIs(t, err, ErrSessionNotFound)

	// Test get non-existent session
	_, err = store.Get(ctx, "non-existent")
	assert.ErrorIs(t, err, ErrSessionNotFound)

	// Test Cleanup
	expiredSession := &Session{
		ID:        "expired-session",
		UserID:    "test-user",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // Expired
		Active:    true,
	}
	inactiveSession := &Session{
		ID:        "inactive-session",
		UserID:    "test-user",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		Active:    false, // Inactive
	}
	validSession := &Session{
		ID:        "valid-session",
		UserID:    "test-user",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		Active:    true,
	}

	// Store all sessions
	require.NoError(t, store.Store(ctx, expiredSession))
	require.NoError(t, store.Store(ctx, inactiveSession))
	require.NoError(t, store.Store(ctx, validSession))

	// Run cleanup
	err = store.Cleanup(ctx)
	require.NoError(t, err)

	// Verify cleanup results
	_, err = store.Get(ctx, expiredSession.ID)
	assert.ErrorIs(t, err, ErrSessionNotFound, "Expired session should be removed")

	_, err = store.Get(ctx, inactiveSession.ID)
	assert.ErrorIs(t, err, ErrSessionNotFound, "Inactive session should be removed")

	_, err = store.Get(ctx, validSession.ID)
	assert.NoError(t, err, "Valid session should remain")
}
