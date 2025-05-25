package auth

import (
	"time"
)

// Config represents the authentication module configuration
type Config struct {
	JWT      JWTConfig      `yaml:"jwt"`
	Session  SessionConfig  `yaml:"session"`
	OAuth2   OAuth2Config   `yaml:"oauth2"`
	Password PasswordConfig `yaml:"password"`
}

// JWTConfig contains JWT-related configuration
type JWTConfig struct {
	Secret            string        `yaml:"secret" required:"true"`
	Expiration        time.Duration `yaml:"expiration" default:"24h"`
	RefreshExpiration time.Duration `yaml:"refresh_expiration" default:"168h"` // 7 days
	Issuer            string        `yaml:"issuer" default:"modular-auth"`
	Algorithm         string        `yaml:"algorithm" default:"HS256"`
}

// SessionConfig contains session-related configuration
type SessionConfig struct {
	Store      string        `yaml:"store" default:"memory"` // memory, redis, database
	CookieName string        `yaml:"cookie_name" default:"session_id"`
	MaxAge     time.Duration `yaml:"max_age" default:"24h"`
	Secure     bool          `yaml:"secure" default:"true"`
	HTTPOnly   bool          `yaml:"http_only" default:"true"`
	SameSite   string        `yaml:"same_site" default:"strict"` // strict, lax, none
	Domain     string        `yaml:"domain"`
	Path       string        `yaml:"path" default:"/"`
}

// OAuth2Config contains OAuth2/OIDC configuration
type OAuth2Config struct {
	Providers map[string]OAuth2Provider `yaml:"providers"`
}

// OAuth2Provider represents an OAuth2 provider configuration
type OAuth2Provider struct {
	ClientID     string   `yaml:"client_id" required:"true"`
	ClientSecret string   `yaml:"client_secret" required:"true"`
	RedirectURL  string   `yaml:"redirect_url" required:"true"`
	Scopes       []string `yaml:"scopes"`
	AuthURL      string   `yaml:"auth_url"`
	TokenURL     string   `yaml:"token_url"`
	UserInfoURL  string   `yaml:"user_info_url"`
}

// PasswordConfig contains password-related configuration
type PasswordConfig struct {
	Algorithm      string `yaml:"algorithm" default:"bcrypt"` // bcrypt, argon2
	MinLength      int    `yaml:"min_length" default:"8"`
	RequireUpper   bool   `yaml:"require_upper" default:"true"`
	RequireLower   bool   `yaml:"require_lower" default:"true"`
	RequireDigit   bool   `yaml:"require_digit" default:"true"`
	RequireSpecial bool   `yaml:"require_special" default:"false"`
	BcryptCost     int    `yaml:"bcrypt_cost" default:"12"`
}

// Validate validates the authentication configuration
func (c *Config) Validate() error {
	if c.JWT.Secret == "" {
		return ErrInvalidConfig
	}

	if c.JWT.Expiration <= 0 {
		return ErrInvalidConfig
	}

	if c.JWT.RefreshExpiration <= 0 {
		return ErrInvalidConfig
	}

	if c.Password.MinLength < 1 {
		return ErrInvalidConfig
	}

	if c.Password.BcryptCost < 4 || c.Password.BcryptCost > 31 {
		return ErrInvalidConfig
	}

	return nil
}
