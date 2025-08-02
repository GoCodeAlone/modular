package auth

import "errors"

// Auth module specific errors
var (
	ErrInvalidConfig           = errors.New("invalid auth configuration")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrTokenExpired            = errors.New("token has expired")
	ErrTokenInvalid            = errors.New("token is invalid")
	ErrTokenMalformed          = errors.New("token is malformed")
	ErrUserNotFound            = errors.New("user not found")
	ErrUserAlreadyExists       = errors.New("user already exists")
	ErrPasswordTooWeak         = errors.New("password does not meet requirements")
	ErrSessionNotFound         = errors.New("session not found")
	ErrSessionExpired          = errors.New("session has expired")
	ErrOAuth2Failed            = errors.New("oauth2 authentication failed")
	ErrProviderNotFound        = errors.New("oauth2 provider not found")
	ErrUserStoreInvalid        = errors.New("user_store service does not implement UserStore interface")
	ErrSessionStoreInvalid     = errors.New("session_store service does not implement SessionStore interface")
	ErrUnexpectedSigningMethod = errors.New("unexpected signing method")
	ErrUserInfoNotConfigured   = errors.New("user info URL not configured for provider")
	ErrRandomGeneration        = errors.New("failed to generate random bytes")
)
