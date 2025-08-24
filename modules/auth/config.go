package auth

import (
	"time"
)

// Config represents the authentication module configuration
type Config struct {
	JWT      JWTConfig      `yaml:"jwt" env:"JWT"`
	Session  SessionConfig  `yaml:"session" env:"SESSION"`
	OAuth2   OAuth2Config   `yaml:"oauth2" env:"OAUTH2"`
	Password PasswordConfig `yaml:"password" env:"PASSWORD"`
}

// JWTConfig contains JWT-related configuration
type JWTConfig struct {
	Secret            string        `yaml:"secret" required:"true" env:"SECRET"`
	Expiration        time.Duration `yaml:"expiration" default:"24h" env:"EXPIRATION"`
	RefreshExpiration time.Duration `yaml:"refresh_expiration" default:"168h" env:"REFRESH_EXPIRATION"` // 7 days
	Issuer            string        `yaml:"issuer" default:"modular-auth" env:"ISSUER"`
	Algorithm         string        `yaml:"algorithm" default:"HS256" env:"ALGORITHM"`
}

// SessionConfig contains session-related configuration
type SessionConfig struct {
	Store      string        `yaml:"store" default:"memory" env:"STORE"` // memory, redis, database
	CookieName string        `yaml:"cookie_name" default:"session_id" env:"COOKIE_NAME"`
	MaxAge     time.Duration `yaml:"max_age" default:"24h" env:"MAX_AGE"`
	Secure     bool          `yaml:"secure" default:"true" env:"SECURE"`
	HTTPOnly   bool          `yaml:"http_only" default:"true" env:"HTTP_ONLY"`
	SameSite   string        `yaml:"same_site" default:"strict" env:"SAME_SITE"` // strict, lax, none
	Domain     string        `yaml:"domain" env:"DOMAIN"`
	Path       string        `yaml:"path" default:"/" env:"PATH"`
}

// OAuth2Config contains OAuth2/OIDC configuration
type OAuth2Config struct {
	Providers map[string]OAuth2Provider `yaml:"providers" env:"PROVIDERS"`
}

// OAuth2Provider represents an OAuth2 provider configuration
type OAuth2Provider struct {
	ClientID     string   `yaml:"client_id" required:"true" env:"CLIENT_ID"`
	ClientSecret string   `yaml:"client_secret" required:"true" env:"CLIENT_SECRET"`
	RedirectURL  string   `yaml:"redirect_url" required:"true" env:"REDIRECT_URL"`
	Scopes       []string `yaml:"scopes" env:"SCOPES"`
	AuthURL      string   `yaml:"auth_url" env:"AUTH_URL"`
	TokenURL     string   `yaml:"token_url" env:"TOKEN_URL"`
	UserInfoURL  string   `yaml:"user_info_url" env:"USER_INFO_URL"`
}

// PasswordConfig contains password-related configuration
type PasswordConfig struct {
	Algorithm      string `yaml:"algorithm" default:"bcrypt" env:"ALGORITHM"` // bcrypt, argon2
	MinLength      int    `yaml:"min_length" default:"8" env:"MIN_LENGTH"`
	RequireUpper   bool   `yaml:"require_upper" default:"true" env:"REQUIRE_UPPER"`
	RequireLower   bool   `yaml:"require_lower" default:"true" env:"REQUIRE_LOWER"`
	RequireDigit   bool   `yaml:"require_digit" default:"true" env:"REQUIRE_DIGIT"`
	RequireSpecial bool   `yaml:"require_special" default:"false" env:"REQUIRE_SPECIAL"`
	BcryptCost     int    `yaml:"bcrypt_cost" default:"12" env:"BCRYPT_COST"`
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

// GetJWTExpiration returns the JWT expiration as time.Duration
func (c *JWTConfig) GetJWTExpiration() time.Duration {
	return c.Expiration
}

// GetJWTRefreshExpiration returns the JWT refresh expiration as time.Duration
func (c *JWTConfig) GetJWTRefreshExpiration() time.Duration {
	return c.RefreshExpiration
}

// GetSessionMaxAge returns the session max age as time.Duration
func (c *SessionConfig) GetSessionMaxAge() time.Duration {
	return c.MaxAge
}
