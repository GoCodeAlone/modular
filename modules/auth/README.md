# Authentication Module

[![Go Reference](https://pkg.go.dev/badge/github.com/CrisisTextLine/modular/modules/auth.svg)](https://pkg.go.dev/github.com/CrisisTextLine/modular/modules/auth)

The Authentication module provides comprehensive authentication capabilities for the Modular framework, including JWT tokens, session management, password hashing, and OAuth2/OIDC integration.

## Features

- **JWT Token Management**: Generate, validate, and refresh JWT tokens with custom claims
- **Password Security**: Secure password hashing using bcrypt with configurable strength requirements
- **Session Management**: Create, manage, and track user sessions with configurable storage backends
- **OAuth2/OIDC Support**: Integration with OAuth2 providers like Google, GitHub, etc.
- **Flexible Storage**: Pluggable user and session storage interfaces with in-memory implementations included
- **Security**: Built-in protection against common authentication vulnerabilities

## Installation

```bash
go get github.com/CrisisTextLine/modular/modules/auth
```

## Configuration

### Basic Configuration

```yaml
auth:
  jwt:
    secret: "your-jwt-secret-key"
    expiration: "24h"
    refresh_expiration: "168h"
    issuer: "your-app-name"
  
  password:
    min_length: 8
    require_upper: true
    require_lower: true
    require_digit: true
    require_special: false
    bcrypt_cost: 12
  
  session:
    store: "memory"
    cookie_name: "session_id"
    max_age: "24h"
    secure: true
    http_only: true
```

### OAuth2 Configuration

```yaml
auth:
  oauth2:
    providers:
      google:
        client_id: "your-google-client-id"
        client_secret: "your-google-client-secret"
        redirect_url: "http://localhost:8080/auth/google/callback"
        scopes: ["openid", "email", "profile"]
        auth_url: "https://accounts.google.com/o/oauth2/auth"
        token_url: "https://oauth2.googleapis.com/token"
        user_info_url: "https://www.googleapis.com/oauth2/v2/userinfo"
```

## Usage

### Basic Setup

```go
package main

import (
    "github.com/CrisisTextLine/modular"
    "github.com/CrisisTextLine/modular/modules/auth"
)

func main() {
    app := modular.NewApplication()
    
    // Register the auth module
    app.RegisterModule(auth.NewModule())
    
    // Start the application
    app.Start()
}
```

### Using the Auth Service

```go
// Get the auth service from the application
var authService auth.AuthService
err := app.GetService(auth.ServiceName, &authService)
if err != nil {
    log.Fatal(err)
}

// Hash a password
hashedPassword, err := authService.HashPassword("userpassword123")
if err != nil {
    log.Fatal(err)
}

// Verify a password
err = authService.VerifyPassword(hashedPassword, "userpassword123")
if err != nil {
    log.Println("Invalid password")
}

// Generate JWT tokens
customClaims := map[string]interface{}{
    "email": "user@example.com",
    "roles": []string{"user", "admin"},
    "permissions": []string{"read", "write"},
}

tokenPair, err := authService.GenerateToken("user-123", customClaims)
if err != nil {
    log.Fatal(err)
}

// Validate a token
claims, err := authService.ValidateToken(tokenPair.AccessToken)
if err != nil {
    log.Println("Invalid token:", err)
    return
}

log.Printf("User ID: %s, Email: %s", claims.UserID, claims.Email)
```

### Session Management

```go
// Create a session
metadata := map[string]interface{}{
    "ip_address": "127.0.0.1",
    "user_agent": "Mozilla/5.0...",
}

session, err := authService.CreateSession("user-123", metadata)
if err != nil {
    log.Fatal(err)
}

// Get a session
retrievedSession, err := authService.GetSession(session.ID)
if err != nil {
    log.Println("Session not found:", err)
    return
}

// Refresh a session (extend expiration)
refreshedSession, err := authService.RefreshSession(session.ID)
if err != nil {
    log.Fatal(err)
}

// Delete a session
err = authService.DeleteSession(session.ID)
if err != nil {
    log.Fatal(err)
}
```

### OAuth2 Integration

```go
// Get OAuth2 authorization URL
state := "random-state-string"
authURL, err := authService.GetOAuth2AuthURL("google", state)
if err != nil {
    log.Fatal(err)
}

// Redirect user to authURL...

// Exchange authorization code for user info
code := "authorization-code-from-callback"
result, err := authService.ExchangeOAuth2Code("google", code, state)
if err != nil {
    log.Fatal(err)
}

log.Printf("OAuth2 Result: %+v", result)
```

### Custom User and Session Stores

You can implement custom storage backends by implementing the `UserStore` and `SessionStore` interfaces:

```go
type DatabaseUserStore struct {
    db *sql.DB
}

func (s *DatabaseUserStore) GetUser(ctx context.Context, userID string) (*auth.User, error) {
    // Implement database query
}

func (s *DatabaseUserStore) CreateUser(ctx context.Context, user *auth.User) error {
    // Implement database insert
}

// Implement other UserStore methods...

// Register custom stores
app.RegisterService("user_store", &DatabaseUserStore{db: db})
app.RegisterService("session_store", &RedisSessionStore{client: redisClient})
```

## API Reference

### AuthService Interface

The main authentication service interface provides the following methods:

#### JWT Operations
- `GenerateToken(userID string, claims map[string]interface{}) (*TokenPair, error)`
- `ValidateToken(token string) (*Claims, error)`
- `RefreshToken(refreshToken string) (*TokenPair, error)`

#### Password Operations
- `HashPassword(password string) (string, error)`
- `VerifyPassword(hashedPassword, password string) error`
- `ValidatePasswordStrength(password string) error`

#### Session Operations
- `CreateSession(userID string, metadata map[string]interface{}) (*Session, error)`
- `GetSession(sessionID string) (*Session, error)`
- `DeleteSession(sessionID string) error`
- `RefreshSession(sessionID string) (*Session, error)`

#### OAuth2 Operations
- `GetOAuth2AuthURL(provider, state string) (string, error)`
- `ExchangeOAuth2Code(provider, code, state string) (*OAuth2Result, error)`

### Data Structures

#### User
```go
type User struct {
    ID           string                 `json:"id"`
    Email        string                 `json:"email"`
    PasswordHash string                 `json:"-"`
    Roles        []string               `json:"roles"`
    Permissions  []string               `json:"permissions"`
    Active       bool                   `json:"active"`
    CreatedAt    time.Time              `json:"created_at"`
    UpdatedAt    time.Time              `json:"updated_at"`
    LastLoginAt  *time.Time             `json:"last_login_at,omitempty"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
```

#### Session
```go
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
```

#### TokenPair
```go
type TokenPair struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    TokenType    string    `json:"token_type"`
    ExpiresIn    int64     `json:"expires_in"`
    ExpiresAt    time.Time `json:"expires_at"`
}
```

## Testing

The module includes comprehensive tests covering all functionality:

```bash
cd modules/auth
go test -v
```

### Test Coverage

- Configuration validation
- JWT token generation and validation
- Password hashing and verification
- Session management
- OAuth2 integration
- Memory store implementations
- Module lifecycle and dependency injection

## Security Considerations

1. **JWT Secret**: Use a strong, randomly generated secret for JWT signing
2. **Password Hashing**: bcrypt cost should be at least 12 for production
3. **Session Security**: Enable secure and httpOnly flags for session cookies
4. **OAuth2**: Validate state parameters to prevent CSRF attacks
5. **Token Expiration**: Set appropriate expiration times for access and refresh tokens

## Examples

See the `examples/` directory for complete usage examples including:

- Basic authentication setup
- OAuth2 integration
- Custom middleware implementation
- Database integration patterns

## Dependencies

- `github.com/golang-jwt/jwt/v5` - JWT token handling
- `golang.org/x/crypto` - Password hashing
- `golang.org/x/oauth2` - OAuth2 client implementation

## License

This module is part of the Modular framework and is licensed under the same terms.