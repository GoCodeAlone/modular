package database

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSpecialCharacterPasswordDSNParsing tests the specific issue from the GitHub issue #19
func TestSpecialCharacterPasswordDSNParsing(t *testing.T) {
	// This is the exact DSN from the GitHub issue
	issueExampleDSN := "postgresql://someuser:8jKwouNHdI!u6a?kx(UuQ-Bgm34P@some-dev-backend.cluster.us-east-1.rds.amazonaws.com/some_backend"
	
	// Test that endpoint extraction works
	endpoint, err := extractEndpointFromDSN(issueExampleDSN)
	require.NoError(t, err)
	require.Equal(t, "some-dev-backend.cluster.us-east-1.rds.amazonaws.com", endpoint)
	
	// Test that password replacement works
	token := "test-iam-token"
	newDSN, err := replaceDSNPassword(issueExampleDSN, token)
	require.NoError(t, err)
	require.Contains(t, newDSN, "postgresql://someuser:test-iam-token@some-dev-backend.cluster.us-east-1.rds.amazonaws.com/some_backend")
	
	// Test that we can create a database service with this DSN (without actually connecting)
	config := ConnectionConfig{
		Driver: "postgres",
		DSN:    issueExampleDSN,
	}
	
	service, err := NewDatabaseService(config)
	require.NoError(t, err)
	require.NotNil(t, service)
	
	// Clean up
	err = service.Close()
	require.NoError(t, err)
}

// TestSpecialCharacterPasswordDSNParsingWithAWSIAM tests the issue with AWS IAM auth
func TestSpecialCharacterPasswordDSNParsingWithAWSIAM(t *testing.T) {
	// This is the exact DSN from the GitHub issue  
	issueExampleDSN := "postgresql://someuser:8jKwouNHdI!u6a?kx(UuQ-Bgm34P@some-dev-backend.cluster.us-east-1.rds.amazonaws.com/some_backend"
	
	// Test that we can create a database service with AWS IAM auth enabled
	config := ConnectionConfig{
		Driver: "postgres",
		DSN:    issueExampleDSN,
		AWSIAMAuth: &AWSIAMAuthConfig{
			Enabled:              true,
			Region:               "us-east-1",
			DBUser:               "someuser",
			TokenRefreshInterval: 300,
		},
	}
	
	// Skip this test if AWS credentials are not available
	service, err := NewDatabaseService(config)
	if err != nil {
		// If AWS config loading fails, skip this test
		if err.Error() == "failed to create AWS IAM token provider: failed to load AWS config: no EC2 IMDS role found, operation error ec2imds: GetMetadata, canceled, context canceled" {
			t.Skip("AWS credentials not available, skipping test")
		}
		t.Fatalf("Failed to create service: %v", err)
	}
	require.NotNil(t, service)
	
	// Clean up
	err = service.Close()
	require.NoError(t, err)
}

// TestEdgeCaseSpecialCharacterPasswords tests various edge cases
func TestEdgeCaseSpecialCharacterPasswords(t *testing.T) {
	testCases := []struct {
		name        string
		dsn         string
		expectedHost string
	}{
		{
			name:        "password with @ symbol",
			dsn:         "postgres://user:pass@word@host.com:5432/db",
			expectedHost: "host.com:5432",
		},
		{
			name:        "password with multiple @ symbols",
			dsn:         "postgres://user:p@ss@w@rd@host.com:5432/db",
			expectedHost: "host.com:5432",
		},
		{
			name:        "password with query-like characters",
			dsn:         "postgres://user:pass?key=value&other=test@host.com:5432/db",
			expectedHost: "host.com:5432",
		},
		{
			name:        "password with URL-like structure",
			dsn:         "postgres://user:http://example.com/path?query=value@host.com:5432/db",
			expectedHost: "host.com:5432",
		},
		{
			name:        "password with colon",
			dsn:         "postgres://user:pass:word@host.com:5432/db",
			expectedHost: "host.com:5432",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint, err := extractEndpointFromDSN(tc.dsn)
			require.NoError(t, err)
			require.Equal(t, tc.expectedHost, endpoint)
			
			// Test password replacement
			token := "test-token"
			newDSN, err := replaceDSNPassword(tc.dsn, token)
			require.NoError(t, err)
			require.Contains(t, newDSN, token)
			
			// Verify we can parse the new DSN
			newEndpoint, err := extractEndpointFromDSN(newDSN)
			require.NoError(t, err)
			require.Equal(t, tc.expectedHost, newEndpoint)
		})
	}
}