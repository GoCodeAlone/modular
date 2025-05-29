package auth

import (
	"context"
	"net/http"
	"time"
)

// AuthService defines the main authentication service interface
type AuthService interface {
	// JWT operations
	GenerateToken(userID string, claims map[string]interface{}) (*TokenPair, error)
	ValidateToken(token string) (*Claims, error)
	RefreshToken(refreshToken string) (*TokenPair, error)

	// Password operations
	HashPassword(password string) (string, error)
	VerifyPassword(hashedPassword, password string) error
	ValidatePasswordStrength(password string) error

	// Session operations
	CreateSession(userID string, metadata map[string]interface{}) (*Session, error)
	GetSession(sessionID string) (*Session, error)
	DeleteSession(sessionID string) error
	RefreshSession(sessionID string) (*Session, error)

	// OAuth2 operations
	GetOAuth2AuthURL(provider, state string) (string, error)
	ExchangeOAuth2Code(provider, code, state string) (*OAuth2Result, error)
}

// UserStore defines the interface for user storage operations
type UserStore interface {
	GetUser(ctx context.Context, userID string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, userID string) error
}

// SessionStore defines the interface for session storage operations
type SessionStore interface {
	Store(ctx context.Context, session *Session) error
	Get(ctx context.Context, sessionID string) (*Session, error)
	Delete(ctx context.Context, sessionID string) error
	Cleanup(ctx context.Context) error // Remove expired sessions
}

// Middleware defines authentication middleware interface
type Middleware interface {
	RequireAuth(next http.Handler) http.Handler
	OptionalAuth(next http.Handler) http.Handler
	RequireRole(role string) func(http.Handler) http.Handler
	RequirePermission(permission string) func(http.Handler) http.Handler
}

// TokenPair represents an access token and refresh token pair
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Claims represents JWT token claims
type Claims struct {
	UserID      string                 `json:"user_id"`
	Email       string                 `json:"email"`
	Roles       []string               `json:"roles"`
	Permissions []string               `json:"permissions"`
	IssuedAt    time.Time              `json:"iat"`
	ExpiresAt   time.Time              `json:"exp"`
	Issuer      string                 `json:"iss"`
	Subject     string                 `json:"sub"`
	Custom      map[string]interface{} `json:"custom,omitempty"`
}

// User represents a user in the authentication system
type User struct {
	ID           string                 `json:"id"`
	Email        string                 `json:"email"`
	PasswordHash string                 `json:"-"` // Never serialize password hash
	Roles        []string               `json:"roles"`
	Permissions  []string               `json:"permissions"`
	Active       bool                   `json:"active"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	LastLoginAt  *time.Time             `json:"last_login_at,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Session represents a user session
type Session struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	CreatedAt time.Time              `json:"created_at"`
	ExpiresAt time.Time              `json:"expires_at"`
	IPAddress string                 `json:"ip_address"`
	UserAgent string                 `json:"user_agent"`
	Active    bool                   `json:"active"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// OAuth2Result represents the result of OAuth2 authentication
type OAuth2Result struct {
	Provider     string                 `json:"provider"`
	UserInfo     map[string]interface{} `json:"user_info"`
	AccessToken  string                 `json:"access_token"`
	RefreshToken string                 `json:"refresh_token"`
	ExpiresAt    time.Time              `json:"expires_at"`
}

// AuthContext represents authentication context in HTTP requests
type AuthContext struct {
	User        *User    `json:"user"`
	Session     *Session `json:"session"`
	Claims      *Claims  `json:"claims"`
	Permissions []string `json:"permissions"`
	Roles       []string `json:"roles"`
}
