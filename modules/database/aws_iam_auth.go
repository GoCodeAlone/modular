package database

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
)

var (
	ErrIAMAuthNotEnabled     = errors.New("AWS IAM auth not enabled")
	ErrIAMRegionRequired     = errors.New("AWS region is required for IAM authentication")
	ErrIAMDBUserRequired     = errors.New("database user is required for IAM authentication")
	ErrExtractEndpointFailed = errors.New("could not extract endpoint from DSN")
	ErrNoUserInfoInDSN       = errors.New("no user information in DSN to replace password")
)

// AWSIAMTokenProvider manages AWS IAM authentication tokens for RDS
type AWSIAMTokenProvider struct {
	config         *AWSIAMAuthConfig
	awsConfig      aws.Config
	currentToken   string
	tokenExpiry    time.Time
	mutex          sync.RWMutex
	stopChan       chan struct{}
	refreshDone    chan struct{}
	refreshStarted bool
}

// NewAWSIAMTokenProvider creates a new AWS IAM token provider
func NewAWSIAMTokenProvider(authConfig *AWSIAMAuthConfig) (*AWSIAMTokenProvider, error) {
	if authConfig == nil || !authConfig.Enabled {
		return nil, ErrIAMAuthNotEnabled
	}

	if authConfig.Region == "" {
		return nil, ErrIAMRegionRequired
	}

	if authConfig.DBUser == "" {
		return nil, ErrIAMDBUserRequired
	}

	// Set default token refresh interval if not specified
	if authConfig.TokenRefreshInterval <= 0 {
		authConfig.TokenRefreshInterval = 600 // 10 minutes
	}

	// Load AWS configuration
	ctx := context.Background()
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(authConfig.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	provider := &AWSIAMTokenProvider{
		config:      authConfig,
		awsConfig:   awsConfig,
		stopChan:    make(chan struct{}),
		refreshDone: make(chan struct{}),
	}

	return provider, nil
}

// GetToken returns the current valid IAM token, refreshing if necessary
func (p *AWSIAMTokenProvider) GetToken(ctx context.Context, endpoint string) (string, error) {
	p.mutex.RLock()
	if p.currentToken != "" && time.Now().Before(p.tokenExpiry) {
		token := p.currentToken
		p.mutex.RUnlock()
		return token, nil
	}
	p.mutex.RUnlock()

	return p.refreshToken(ctx, endpoint)
}

// refreshToken generates a new IAM authentication token
func (p *AWSIAMTokenProvider) refreshToken(ctx context.Context, endpoint string) (string, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Double-check in case another goroutine already refreshed the token
	if p.currentToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.currentToken, nil
	}

	// Generate new token
	token, err := auth.BuildAuthToken(ctx, endpoint, p.config.Region, p.config.DBUser, p.awsConfig.Credentials)
	if err != nil {
		return "", fmt.Errorf("failed to build AWS IAM auth token: %w", err)
	}

	p.currentToken = token
	// Tokens are valid for 15 minutes, we refresh earlier to avoid expiry
	p.tokenExpiry = time.Now().Add(time.Duration(p.config.TokenRefreshInterval) * time.Second)

	return token, nil
}

// StartTokenRefresh starts a background goroutine to refresh tokens periodically
func (p *AWSIAMTokenProvider) StartTokenRefresh(ctx context.Context, endpoint string) {
	p.mutex.Lock()
	if p.refreshStarted {
		p.mutex.Unlock()
		return
	}
	p.refreshStarted = true
	p.mutex.Unlock()

	go p.tokenRefreshLoop(ctx, endpoint)
}

// StopTokenRefresh stops the background token refresh
func (p *AWSIAMTokenProvider) StopTokenRefresh() {
	p.mutex.Lock()
	if !p.refreshStarted {
		p.mutex.Unlock()
		return
	}
	p.mutex.Unlock()

	close(p.stopChan)
	<-p.refreshDone
}

// tokenRefreshLoop runs in the background to refresh tokens
func (p *AWSIAMTokenProvider) tokenRefreshLoop(ctx context.Context, endpoint string) {
	defer close(p.refreshDone)

	ticker := time.NewTicker(time.Duration(p.config.TokenRefreshInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			_, err := p.refreshToken(ctx, endpoint)
			if err != nil {
				// Log error but continue trying to refresh
				// In a real implementation, you might want to use the module's logger
				fmt.Printf("Failed to refresh AWS IAM token: %v\n", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// BuildDSNWithIAMToken takes a DSN and replaces the password with the IAM token
func (p *AWSIAMTokenProvider) BuildDSNWithIAMToken(ctx context.Context, originalDSN string) (string, error) {
	endpoint, err := extractEndpointFromDSN(originalDSN)
	if err != nil {
		return "", fmt.Errorf("failed to extract endpoint from DSN: %w", err)
	}

	token, err := p.GetToken(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to get IAM token: %w", err)
	}

	return replaceDSNPassword(originalDSN, token)
}

// extractEndpointFromDSN extracts the database endpoint from a DSN
func extractEndpointFromDSN(dsn string) (string, error) {
	// Handle different DSN formats
	if strings.Contains(dsn, "://") {
		// URL-style DSN (e.g., postgres://user:password@host:port/database)
		// Handle potential special characters in password by preprocessing
		preprocessedDSN, err := preprocessDSNForParsing(dsn)
		if err != nil {
			return "", fmt.Errorf("failed to preprocess DSN: %w", err)
		}

		u, err := url.Parse(preprocessedDSN)
		if err != nil {
			return "", fmt.Errorf("failed to parse DSN URL: %w", err)
		}
		return u.Host, nil
	}

	// Key-value style DSN (e.g., host=localhost port=5432 user=postgres)
	parts := strings.Fields(dsn)
	for _, part := range parts {
		if strings.HasPrefix(part, "host=") {
			host := strings.TrimPrefix(part, "host=")
			// Look for port in the same DSN
			for _, p := range parts {
				if strings.HasPrefix(p, "port=") {
					port := strings.TrimPrefix(p, "port=")
					return host + ":" + port, nil
				}
			}
			return host + ":5432", nil // Default PostgreSQL port
		}
	}

	return "", ErrExtractEndpointFailed
}

// preprocessDSNForParsing handles special characters in passwords by URL-encoding them
func preprocessDSNForParsing(dsn string) (string, error) {
	// Find the pattern: ://username:password@host
	protocolEnd := strings.Index(dsn, "://")
	if protocolEnd == -1 {
		return dsn, nil // Not a URL-style DSN
	}

	// Find the start of credentials (after ://)
	credentialsStart := protocolEnd + 3

	// Find the end of credentials (before @host)
	// We need to find the correct @ that separates credentials from host
	// Look for the pattern @host:port or @host/path or @host (end of string)
	remainingDSN := dsn[credentialsStart:]

	// Find the @ that is followed by a valid hostname pattern
	// A hostname should not contain most special characters that would be in a password
	// Search from right to left to find the last @ that's followed by a hostname
	var atIndex = -1
	for i := len(remainingDSN) - 1; i >= 0; i-- {
		if remainingDSN[i] == '@' {
			// Check if what follows looks like a hostname
			hostPart := remainingDSN[i+1:]
			if len(hostPart) > 0 && looksLikeHostname(hostPart) {
				atIndex = i
				break
			}
		}
	}

	if atIndex == -1 {
		return dsn, nil // No credentials
	}

	// Extract the credentials part
	credentialsEnd := credentialsStart + atIndex
	credentials := dsn[credentialsStart:credentialsEnd]

	// Find the colon that separates username from password
	colonIndex := strings.Index(credentials, ":")
	if colonIndex == -1 {
		return dsn, nil // No password
	}

	// Extract username and password
	username := credentials[:colonIndex]
	password := credentials[colonIndex+1:]

	// Check if password is already URL-encoded
	// A properly URL-encoded password should contain % characters followed by hex digits
	isAlreadyEncoded := strings.Contains(password, "%") && func() bool {
		// Check if it contains URL-encoded patterns like %20, %21, etc.
		for i := 0; i < len(password)-2; i++ {
			if password[i] == '%' {
				// Check if the next two characters are hex digits
				if len(password) > i+2 {
					c1, c2 := password[i+1], password[i+2]
					if isHexDigit(c1) && isHexDigit(c2) {
						return true
					}
				}
			}
		}
		return false
	}()

	if isAlreadyEncoded {
		// Password is already encoded, return as-is
		return dsn, nil
	}

	// URL-encode the password
	encodedPassword := url.QueryEscape(password)

	// Reconstruct the DSN with encoded password
	encodedDSN := dsn[:credentialsStart] + username + ":" + encodedPassword + dsn[credentialsEnd:]

	return encodedDSN, nil
}

// isHexDigit checks if a character is a hexadecimal digit
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

// looksLikeHostname checks if a string looks like a hostname
func looksLikeHostname(hostPart string) bool {
	// Split by / to get just the host:port part (before any path)
	parts := strings.SplitN(hostPart, "/", 2)
	hostAndPort := parts[0]

	// Split by ? to get just the host:port part (before any query params)
	parts = strings.SplitN(hostAndPort, "?", 2)
	hostAndPort = parts[0]

	if len(hostAndPort) == 0 {
		return false
	}

	// Check if it contains characters that are unlikely to be in hostnames
	// but common in passwords
	for _, char := range hostAndPort {
		// These characters are not typically found in hostnames
		if char == '!' || char == '#' || char == '$' || char == '%' ||
			char == '^' || char == '&' || char == '*' || char == '(' ||
			char == ')' || char == '+' || char == '=' || char == '[' ||
			char == ']' || char == '{' || char == '}' || char == '|' ||
			char == ';' || char == '\'' || char == '"' || char == ',' ||
			char == '<' || char == '>' || char == '\\' {
			return false
		}
	}

	// Additional checks: hostname should contain at least one dot or be localhost
	// and should not start with special characters
	return (strings.Contains(hostAndPort, ".") || hostAndPort == "localhost" ||
		strings.Contains(hostAndPort, ":")) &&
		(len(hostAndPort) > 0 && (hostAndPort[0] >= 'a' && hostAndPort[0] <= 'z') ||
			(hostAndPort[0] >= 'A' && hostAndPort[0] <= 'Z') ||
			(hostAndPort[0] >= '0' && hostAndPort[0] <= '9'))
}

// replaceDSNPassword replaces the password in a DSN with the provided token
func replaceDSNPassword(dsn, token string) (string, error) {
	if strings.Contains(dsn, "://") {
		// URL-style DSN
		// Handle potential special characters in password by preprocessing
		preprocessedDSN, err := preprocessDSNForParsing(dsn)
		if err != nil {
			return "", fmt.Errorf("failed to preprocess DSN: %w", err)
		}

		u, err := url.Parse(preprocessedDSN)
		if err != nil {
			return "", fmt.Errorf("failed to parse DSN URL: %w", err)
		}

		if u.User != nil {
			username := u.User.Username()
			u.User = url.UserPassword(username, token)
		} else {
			return "", ErrNoUserInfoInDSN
		}

		return u.String(), nil
	}

	// Key-value style DSN
	parts := strings.Fields(dsn)
	var result []string
	passwordReplaced := false

	for _, part := range parts {
		if strings.HasPrefix(part, "password=") {
			result = append(result, "password="+token)
			passwordReplaced = true
		} else {
			result = append(result, part)
		}
	}

	if !passwordReplaced {
		result = append(result, "password="+token)
	}

	return strings.Join(result, " "), nil
}
