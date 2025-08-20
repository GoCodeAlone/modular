package auth

import "errors"

// Auth module specific errors
var (
	ErrInvalidConfig             = errors.New("invalid auth configuration")
	ErrInvalidCredentials        = errors.New("invalid credentials")
	ErrTokenExpired              = errors.New("token has expired")
	ErrTokenInvalid              = errors.New("token is invalid")
	ErrTokenMalformed            = errors.New("token is malformed")
	ErrUserNotFound              = errors.New("user not found")
	ErrUserAlreadyExists         = errors.New("user already exists")
	ErrPasswordTooWeak           = errors.New("password does not meet requirements")
	ErrSessionNotFound           = errors.New("session not found")
	ErrSessionExpired            = errors.New("session has expired")
	ErrOAuth2Failed              = errors.New("oauth2 authentication failed")
	ErrProviderNotFound          = errors.New("oauth2 provider not found")
	ErrUnexpectedSigningMethod   = errors.New("unexpected signing method")
	ErrUserStoreNotInterface     = errors.New("user_store service does not implement UserStore interface")
	ErrSessionStoreNotInterface  = errors.New("session_store service does not implement SessionStore interface")
	ErrUserInfoURLNotConfigured  = errors.New("user info URL not configured for provider")
	ErrNoSubjectForEventEmission = errors.New("no subject available for event emission")
)

// UserInfoError represents an error from user info API calls
type UserInfoError struct {
	StatusCode int
	Body       string
}

func (e *UserInfoError) Error() string {
	return "user info request failed"
}
