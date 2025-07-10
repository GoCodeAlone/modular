package database

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSpecialCharacterPasswordDSNParsing tests the specific issue from the GitHub issue #19
func TestSpecialCharacterPasswordDSNParsing(t *testing.T) {
	// This is the exact DSN from the GitHub issue
	issueExampleDSN := "postgresql://someuser:7aBcdeFGhj!r7b?jk(OoL-Aen34R@some-dev-backend.cluster.us-east-1.rds.amazonaws.com/some_backend"

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
	issueExampleDSN := "postgresql://someuser:7aBcdeFGhj!r7b?jk(OoL-Aen34R@some-dev-backend.cluster.us-east-1.rds.amazonaws.com/some_backend"

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
		name         string
		dsn          string
		expectedHost string
	}{
		{
			name:         "password with @ symbol",
			dsn:          "postgres://user:pass@word@host.com:5432/db",
			expectedHost: "host.com:5432",
		},
		{
			name:         "password with multiple @ symbols",
			dsn:          "postgres://user:p@ss@w@rd@host.com:5432/db",
			expectedHost: "host.com:5432",
		},
		{
			name:         "password with query-like characters",
			dsn:          "postgres://user:pass?key=value&other=test@host.com:5432/db",
			expectedHost: "host.com:5432",
		},
		{
			name:         "password with URL-like structure",
			dsn:          "postgres://user:http://example.com/path?query=value@host.com:5432/db",
			expectedHost: "host.com:5432",
		},
		{
			name:         "password with colon",
			dsn:          "postgres://user:pass:word@host.com:5432/db",
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

// TestExactFailingScenario reproduces the exact error from the GitHub issue
func TestExactFailingScenario(t *testing.T) {
	// This is the exact DSN that's causing the error
	problematicDSN := "postgresql://someuser:7aBcdeFGhj!r7b?jk(OoL-Aen34R@cluster.us-east-1.rds.amazonaws.com/someapp"

	// Debug: test preprocessing directly
	preprocessed, err := preprocessDSNForParsing(problematicDSN)
	t.Logf("Original DSN: %s", problematicDSN)
	t.Logf("Preprocessed DSN: %s", preprocessed)
	require.NoError(t, err, "preprocessDSNForParsing should not fail")

	// Test that endpoint extraction works without error
	endpoint, err := extractEndpointFromDSN(problematicDSN)
	require.NoError(t, err, "extractEndpointFromDSN should not fail with special characters in password")
	require.Equal(t, "cluster.us-east-1.rds.amazonaws.com", endpoint)

	// Test that password replacement works
	token := "test-iam-token"
	newDSN, err := replaceDSNPassword(problematicDSN, token)
	require.NoError(t, err, "replaceDSNPassword should not fail with special characters in password")
	require.Contains(t, newDSN, "postgresql://someuser:test-iam-token@cluster.us-east-1.rds.amazonaws.com/someapp")

	// Test that we can create a database service with this DSN (without actually connecting)
	config := ConnectionConfig{
		Driver: "postgres",
		DSN:    problematicDSN,
	}

	service, err := NewDatabaseService(config)
	require.NoError(t, err, "NewDatabaseService should not fail with special characters in DSN")
	require.NotNil(t, service)

	// Clean up
	err = service.Close()
	require.NoError(t, err)
}

// TestDirectURLParsingFailure reproduces the URL parsing error directly
func TestDirectURLParsingFailure(t *testing.T) {
	// This is the exact DSN that's causing the error
	problematicDSN := "postgresql://someuser:7aBcdeFGhj!r7b?jk(OoL-Aen34R@cluster.us-east-1.rds.amazonaws.com/someapp"

	// Test direct URL parsing without preprocessing - this should fail
	_, err := url.Parse(problematicDSN)
	require.Error(t, err, "Direct URL parsing should fail with unencoded special characters")
	require.Contains(t, err.Error(), "invalid port", "Error should mention invalid port")

	// Test URL parsing with preprocessing - this should work
	preprocessed, err := preprocessDSNForParsing(problematicDSN)
	require.NoError(t, err)

	_, err = url.Parse(preprocessed)
	require.NoError(t, err, "URL parsing should work after preprocessing")
}

// TestDSNParsingWithoutAWSIAM tests DSN parsing when AWS IAM is not enabled
func TestDSNParsingWithoutAWSIAM(t *testing.T) {
	// This is the exact DSN that's causing the error
	problematicDSN := "postgresql://someuser:7aBcdeFGhj!r7b?jk(OoL-Aen34R@cluster.us-east-1.rds.amazonaws.com/someapp"

	// Test creating a service without AWS IAM auth
	config := ConnectionConfig{
		Driver: "postgres",
		DSN:    problematicDSN,
		// No AWSIAMAuth - this should still work
	}

	service, err := NewDatabaseService(config)
	require.NoError(t, err, "NewDatabaseService should not fail with special characters in DSN")
	require.NotNil(t, service)

	// The issue would occur when trying to Connect() because sql.Open() would fail
	// For this test, we just verify the service creation and cleanup
	err = service.Close()
	require.NoError(t, err)
}

// TestNonAWSIAMSpecialCharsDSNConnection tests that we can handle special characters
// in passwords even when not using AWS IAM authentication
func TestNonAWSIAMSpecialCharsDSNConnection(t *testing.T) {
	// This is the exact DSN that's causing the error - without AWS IAM auth
	problematicDSN := "postgresql://someuser:7aBcdeFGhj!r7b?jk(OoL-Aen34R@cluster.us-east-1.rds.amazonaws.com/someapp"

	config := ConnectionConfig{
		Driver: "postgres", // Use postgres driver which should be available for testing
		DSN:    problematicDSN,
		// No AWS IAM auth configured - this should trigger the bug
	}

	service, err := NewDatabaseService(config)
	require.NoError(t, err, "NewDatabaseService should not fail")
	require.NotNil(t, service)

	// The actual bug occurs during Connect() when the DSN is passed to sql.Open()
	// This should fail with the original issue if the DSN isn't preprocessed
	err = service.Connect()

	// We expect this to fail due to connection issues (no real database),
	// but it should NOT fail due to DSN parsing issues
	if err != nil {
		// If it fails, it should NOT be a DSN parsing error
		require.NotContains(t, err.Error(), "invalid port",
			"Should not fail with DSN parsing error - indicates special characters not handled")
		require.NotContains(t, err.Error(), "parse",
			"Should not fail with URL parsing error - indicates special characters not handled")
	}

	// Clean up
	err = service.Close()
	require.NoError(t, err)
}

// TestActualURLParsingFailure tests the exact failure scenario that occurs in postgres driver
func TestActualURLParsingFailure(t *testing.T) {
	// This is the exact DSN that's causing the error
	problematicDSN := "postgresql://someuser:7aBcdeFGhj!r7b?jk(OoL-Aen34R@cluster.us-east-1.rds.amazonaws.com/someapp"

	// This is what happens inside the postgres driver when it tries to parse the DSN
	// The driver uses Go's url.Parse() which fails on unencoded special characters
	_, err := url.Parse(problematicDSN)
	require.Error(t, err, "URL parsing should fail with unencoded special characters")
	require.Contains(t, err.Error(), "invalid port", "Should fail with invalid port error")
	require.Contains(t, err.Error(), ":8jKwouNHdI!u6a", "Should show the problematic port parsing")

	t.Logf("Got expected error: %v", err)

	// Try with preprocessed DSN - this should work
	preprocessed, err := preprocessDSNForParsing(problematicDSN)
	require.NoError(t, err)
	t.Logf("Preprocessed DSN: %s", preprocessed)

	_, err = url.Parse(preprocessed)
	require.NoError(t, err, "URL parsing should work with preprocessed DSN")
}

// TestServiceConnectWithoutPreprocessing tests the scenario where the DSN
// is passed directly to the database service without preprocessing
func TestServiceConnectWithoutPreprocessing(t *testing.T) {
	// This is the exact DSN that causes URL parsing errors
	problematicDSN := "postgresql://someuser:7aBcdeFGhj!r7b?jk(OoL-Aen34R@cluster.us-east-1.rds.amazonaws.com/someapp"

	// Confirm that this DSN fails with direct URL parsing (the root cause)
	_, err := url.Parse(problematicDSN)
	require.Error(t, err, "Direct URL parsing should fail")
	require.Contains(t, err.Error(), "invalid port")

	// This represents what would happen in a postgres driver when it tries to parse the DSN
	t.Logf("Expected URL parsing error: %v", err)

	// Now test our service without AWS IAM auth
	// The bug would be that Connect() passes the unprocessed DSN to sql.Open()
	config := ConnectionConfig{
		Driver: "sqlite", // Use sqlite to avoid postgres driver dependency issues
		DSN:    problematicDSN,
		// No AWS IAM auth - this is the scenario where the bug occurs
	}

	service, err := NewDatabaseService(config)
	require.NoError(t, err, "NewDatabaseService should succeed")

	// Clean up
	err = service.Close()
	require.NoError(t, err)
}

// TestDoubleEncodingIssue tests for potential double-encoding when AWS IAM is enabled
func TestDoubleEncodingIssue(t *testing.T) {
	// DSN with special characters that need encoding
	problematicDSN := "postgresql://someuser:7aBcdeFGhj!r7b?jk(OoL-Aen34R@cluster.us-east-1.rds.amazonaws.com/someapp"

	// Test preprocessing once
	preprocessed1, err := preprocessDSNForParsing(problematicDSN)
	require.NoError(t, err)
	t.Logf("First preprocessing: %s", preprocessed1)

	// Test preprocessing twice (this would happen with current Connect() method when AWS IAM is enabled)
	preprocessed2, err := preprocessDSNForParsing(preprocessed1)
	require.NoError(t, err)
	t.Logf("Second preprocessing: %s", preprocessed2)

	// They should be the same (idempotent) - if not, we have double-encoding
	require.Equal(t, preprocessed1, preprocessed2, "Preprocessing should be idempotent - no double-encoding")
}

// TestDebugComplexPassword debugs the complex password parsing issue
func TestDebugComplexPassword(t *testing.T) {
	// The failing DSN from the existing test
	dsn := "postgres://user:p@ssw0rd!#$^&*()_+-=[]{}|;':\",./<>@host.example.com:5432/db"

	t.Logf("Original DSN: %s", dsn)

	// Debug the algorithm step by step
	protocolEnd := strings.Index(dsn, "://")
	credentialsStart := protocolEnd + 3
	remainingDSN := dsn[credentialsStart:]
	t.Logf("Remaining DSN after protocol: %s", remainingDSN)

	// Find all @ positions and check which one we would select
	var selectedAtIndex = -1
	for i := len(remainingDSN) - 1; i >= 0; i-- {
		if remainingDSN[i] == '@' {
			hostPart := remainingDSN[i+1:]
			isHostname := looksLikeHostname(hostPart)
			t.Logf("@ at position %d, hostPart: '%s', looksLikeHostname: %v", i, hostPart, isHostname)
			if isHostname && selectedAtIndex == -1 {
				selectedAtIndex = i
				t.Logf("Selected @ at position %d", i)
			}
		}
	}

	if selectedAtIndex != -1 {
		credentialsEnd := credentialsStart + selectedAtIndex
		credentials := dsn[credentialsStart:credentialsEnd]
		t.Logf("Credentials: '%s'", credentials)

		colonIndex := strings.Index(credentials, ":")
		if colonIndex != -1 {
			username := credentials[:colonIndex]
			password := credentials[colonIndex+1:]
			t.Logf("Username: '%s', Password: '%s'", username, password)

			// Check if already encoded
			decodedPassword, err := url.QueryUnescape(password)
			t.Logf("Password decode attempt: '%s', error: %v", decodedPassword, err)
			isAlreadyEncoded := err == nil && decodedPassword != password
			t.Logf("Is password already encoded? %v", isAlreadyEncoded)
		}
	}

	// Test preprocessing
	processed, err := preprocessDSNForParsing(dsn)
	t.Logf("Processed DSN: %s", processed)
	require.NoError(t, err)

	// Test endpoint extraction
	endpoint, err := extractEndpointFromDSN(dsn)
	t.Logf("Extracted endpoint: %s", endpoint)
	if err != nil {
		t.Logf("Endpoint extraction error: %v", err)
	}
}
