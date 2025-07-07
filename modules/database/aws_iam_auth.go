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
	// We need to find the last @ that separates credentials from host
	// Look for the pattern @host:port or @host/path
	remainingDSN := dsn[credentialsStart:]

	// Find all @ characters
	atIndices := []int{}
	for i := 0; i < len(remainingDSN); i++ {
		if remainingDSN[i] == '@' {
			atIndices = append(atIndices, i)
		}
	}

	if len(atIndices) == 0 {
		return dsn, nil // No credentials
	}

	// Use the last @ as the separator between credentials and host
	atIndex := atIndices[len(atIndices)-1]

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

	// URL-encode the password
	encodedPassword := url.QueryEscape(password)

	// Reconstruct the DSN with encoded password
	encodedDSN := dsn[:credentialsStart] + username + ":" + encodedPassword + dsn[credentialsEnd:]

	return encodedDSN, nil
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
