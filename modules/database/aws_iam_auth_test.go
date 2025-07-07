package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAWSIAMAuthConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *AWSIAMAuthConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "disabled config",
			config: &AWSIAMAuthConfig{
				Enabled: false,
			},
			wantErr: true,
		},
		{
			name: "missing region",
			config: &AWSIAMAuthConfig{
				Enabled: true,
				DBUser:  "testuser",
			},
			wantErr: true,
		},
		{
			name: "missing db user",
			config: &AWSIAMAuthConfig{
				Enabled: true,
				Region:  "us-east-1",
			},
			wantErr: true,
		},
		{
			name: "valid config with defaults",
			config: &AWSIAMAuthConfig{
				Enabled: true,
				Region:  "us-east-1",
				DBUser:  "testuser",
			},
			wantErr: false,
		},
		{
			name: "valid config with custom refresh interval",
			config: &AWSIAMAuthConfig{
				Enabled:              true,
				Region:               "us-east-1",
				DBUser:               "testuser",
				TokenRefreshInterval: 300,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewAWSIAMTokenProvider(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, provider)
			} else {
				require.NoError(t, err)
				require.NotNil(t, provider)

				// Check that default token refresh interval is set
				if tt.config.TokenRefreshInterval <= 0 {
					require.Equal(t, 600, tt.config.TokenRefreshInterval)
				}
			}
		})
	}
}

func TestExtractEndpointFromDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected string
		wantErr  bool
	}{
		{
			name:     "postgres URL style",
			dsn:      "postgres://user:password@mydb.cluster-xyz.us-east-1.rds.amazonaws.com:5432/mydb",
			expected: "mydb.cluster-xyz.us-east-1.rds.amazonaws.com:5432",
			wantErr:  false,
		},
		{
			name:     "mysql URL style",
			dsn:      "mysql://user:password@mydb.cluster-xyz.us-east-1.rds.amazonaws.com:3306/mydb",
			expected: "mydb.cluster-xyz.us-east-1.rds.amazonaws.com:3306",
			wantErr:  false,
		},
		{
			name:     "postgres key-value style with port",
			dsn:      "host=mydb.cluster-xyz.us-east-1.rds.amazonaws.com port=5432 user=postgres dbname=mydb",
			expected: "mydb.cluster-xyz.us-east-1.rds.amazonaws.com:5432",
			wantErr:  false,
		},
		{
			name:     "postgres key-value style without port",
			dsn:      "host=mydb.cluster-xyz.us-east-1.rds.amazonaws.com user=postgres dbname=mydb",
			expected: "mydb.cluster-xyz.us-east-1.rds.amazonaws.com:5432",
			wantErr:  false,
		},
		{
			name:     "postgres URL style with special characters in password",
			dsn:      "postgresql://someuser:8jKwouNHdI!u6a?kx(UuQ-Bgm34P@some-dev-backend.cluster.us-east-1.rds.amazonaws.com/some_backend",
			expected: "some-dev-backend.cluster.us-east-1.rds.amazonaws.com",
			wantErr:  false,
		},
		{
			name:     "postgres URL style with URL-encoded special characters in password",
			dsn:      "postgresql://someuser:8jKwouNHdI%21u6a%3Fkx%28UuQ-Bgm34P@some-dev-backend.cluster.us-east-1.rds.amazonaws.com/some_backend",
			expected: "some-dev-backend.cluster.us-east-1.rds.amazonaws.com",
			wantErr:  false,
		},
		{
			name:     "postgres URL style with complex special characters in password",
			dsn:      "postgres://user:p@ssw0rd!#$^&*()_+-=[]{}|;':\",./<>@host.example.com:5432/db",
			expected: "host.example.com:5432",
			wantErr:  false,
		},
		{
			name:    "invalid DSN",
			dsn:     "invalid-dsn",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, err := extractEndpointFromDSN(tt.dsn)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, endpoint)
			}
		})
	}
}

func TestReplaceDSNPassword(t *testing.T) {
	token := "test-iam-token"

	tests := []struct {
		name     string
		dsn      string
		expected string
		wantErr  bool
	}{
		{
			name:     "postgres URL style",
			dsn:      "postgres://user:oldpassword@host:5432/mydb",
			expected: "postgres://user:test-iam-token@host:5432/mydb",
			wantErr:  false,
		},
		{
			name:     "mysql URL style",
			dsn:      "mysql://user:oldpassword@host:3306/mydb",
			expected: "mysql://user:test-iam-token@host:3306/mydb",
			wantErr:  false,
		},
		{
			name:     "postgres key-value style with existing password",
			dsn:      "host=localhost port=5432 user=postgres password=oldpassword dbname=mydb",
			expected: "host=localhost port=5432 user=postgres password=test-iam-token dbname=mydb",
			wantErr:  false,
		},
		{
			name:     "postgres key-value style without password",
			dsn:      "host=localhost port=5432 user=postgres dbname=mydb",
			expected: "host=localhost port=5432 user=postgres dbname=mydb password=test-iam-token",
			wantErr:  false,
		},
		{
			name:     "postgres URL style with special characters in password",
			dsn:      "postgresql://someuser:8jKwouNHdI!u6a?kx(UuQ-Bgm34P@some-dev-backend.cluster.us-east-1.rds.amazonaws.com/some_backend",
			expected: "postgresql://someuser:test-iam-token@some-dev-backend.cluster.us-east-1.rds.amazonaws.com/some_backend",
			wantErr:  false,
		},
		{
			name:     "postgres URL style with complex special characters in password",
			dsn:      "postgres://user:p@ssw0rd!#$^&*()_+-=[]{}|;':\",./<>@host.example.com:5432/db",
			expected: "postgres://user:test-iam-token@host.example.com:5432/db",
			wantErr:  false,
		},
		{
			name:    "URL style without user info",
			dsn:     "postgres://host:5432/mydb",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := replaceDSNPassword(tt.dsn, token)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDatabaseServiceWithAWSIAMAuth(t *testing.T) {
	// Test creating a database service with AWS IAM auth configuration
	// Note: This test doesn't actually connect to AWS or the database
	config := ConnectionConfig{
		Driver: "postgres",
		DSN:    "postgres://testuser@mydb.cluster-xyz.us-east-1.rds.amazonaws.com:5432/mydb",
		AWSIAMAuth: &AWSIAMAuthConfig{
			Enabled:              true,
			Region:               "us-east-1",
			DBUser:               "testuser",
			TokenRefreshInterval: 300,
		},
	}

	// Skip this test if AWS credentials are not available
	// The test will create the service but not actually connect
	service, err := NewDatabaseService(config)
	if err != nil {
		// If AWS config loading fails, skip this test
		if strings.Contains(err.Error(), "failed to load AWS config") {
			t.Skip("AWS credentials not available, skipping test")
		}
		t.Fatalf("Failed to create service: %v", err)
	}
	require.NotNil(t, service)

	// Cast to implementation to check internal state
	impl, ok := service.(*databaseServiceImpl)
	require.True(t, ok)
	require.NotNil(t, impl.awsTokenProvider)
	require.NotNil(t, impl.ctx)
	require.NotNil(t, impl.cancel)

	// Clean up - this should not hang
	err = service.Close()
	require.NoError(t, err)
}

func TestDatabaseServiceWithoutAWSIAMAuth(t *testing.T) {
	// Test creating a database service without AWS IAM auth
	config := ConnectionConfig{
		Driver: "postgres",
		DSN:    "postgres://user:password@localhost:5432/mydb",
	}

	service, err := NewDatabaseService(config)
	require.NoError(t, err)
	require.NotNil(t, service)

	// Cast to implementation to check internal state
	impl, ok := service.(*databaseServiceImpl)
	require.True(t, ok)
	require.Nil(t, impl.awsTokenProvider)
	require.NotNil(t, impl.ctx)
	require.NotNil(t, impl.cancel)

	// Clean up
	err = service.Close()
	require.NoError(t, err)
}

func TestAWSIAMTokenProvider_NoDeadlockOnClose(t *testing.T) {
	// Test that StopTokenRefresh doesn't deadlock when called before StartTokenRefresh
	config := &AWSIAMAuthConfig{
		Enabled:              true,
		Region:               "us-east-1",
		DBUser:               "testuser",
		TokenRefreshInterval: 300,
	}

	provider, err := NewAWSIAMTokenProvider(config)
	if err != nil {
		if strings.Contains(err.Error(), "failed to load AWS config") {
			t.Skip("AWS credentials not available, skipping test")
		}
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Call StopTokenRefresh before StartTokenRefresh - should not hang
	done := make(chan struct{})
	go func() {
		provider.StopTokenRefresh()
		close(done)
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("StopTokenRefresh() deadlocked when called before StartTokenRefresh()")
	}
}

func TestAWSIAMTokenProvider_StartStopRefresh(t *testing.T) {
	// Test normal start/stop cycle
	config := &AWSIAMAuthConfig{
		Enabled:              true,
		Region:               "us-east-1",
		DBUser:               "testuser",
		TokenRefreshInterval: 1, // Short interval for testing
	}

	provider, err := NewAWSIAMTokenProvider(config)
	if err != nil {
		if strings.Contains(err.Error(), "failed to load AWS config") {
			t.Skip("AWS credentials not available, skipping test")
		}
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start token refresh
	provider.StartTokenRefresh(ctx, "test-endpoint:5432")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Stop token refresh - should not hang
	done := make(chan struct{})
	go func() {
		provider.StopTokenRefresh()
		close(done)
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("StopTokenRefresh() deadlocked")
	}
}

// MockAWSIAMTokenProvider is a mock implementation for testing
type MockAWSIAMTokenProvider struct {
	token     string
	buildErr  error
	getErr    error
	callCount int
}

func (m *MockAWSIAMTokenProvider) BuildDSNWithIAMToken(ctx context.Context, originalDSN string) (string, error) {
	m.callCount++
	if m.buildErr != nil {
		return "", m.buildErr
	}
	return replaceDSNPassword(originalDSN, m.token)
}

func (m *MockAWSIAMTokenProvider) GetToken(ctx context.Context, endpoint string) (string, error) {
	m.callCount++
	if m.getErr != nil {
		return "", m.getErr
	}
	return m.token, nil
}

func (m *MockAWSIAMTokenProvider) StartTokenRefresh(ctx context.Context, endpoint string) {
	// No-op for testing
}

func (m *MockAWSIAMTokenProvider) StopTokenRefresh() {
	// No-op for testing
}
