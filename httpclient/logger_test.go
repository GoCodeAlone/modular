package httpclient

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSanitizeForFilename tests the sanitizeForFilename function with various dangerous inputs
func TestSanitizeForFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		desc     string
	}{
		{
			name:     "normal URL",
			input:    "https://example.com/api/users",
			expected: "https___example.com_api_users",
			desc:     "should replace slashes and colons with underscores",
		},
		{
			name:     "directory traversal with ..",
			input:    "../../../etc/passwd",
			expected: "______etc_passwd",
			desc:     "should replace .. sequences and path separators",
		},
		{
			name:     "multiple .. sequences",
			input:    "test..test..test",
			expected: "test_test_test",
			desc:     "should replace all .. sequences",
		},
		{
			name:     "windows path separators",
			input:    "C:\\Windows\\System32\\config",
			expected: "C__Windows_System32_config",
			desc:     "should replace backslashes with underscores",
		},
		{
			name:     "mixed path separators",
			input:    "/var/www/../../../etc/shadow",
			expected: "_var_www_______etc_shadow",
			desc:     "should handle mixed forward slashes and .. sequences",
		},
		{
			name:     "special filename characters",
			input:    "test<>:|?*file.txt",
			expected: "test______file.txt",
			desc:     "should replace special characters that are invalid in filenames",
		},
		{
			name:     "null byte attempt",
			input:    "test\x00file",
			expected: "test_file",
			desc:     "should replace null bytes with underscores",
		},
		{
			name:     "only invalid characters",
			input:    "../../..",
			expected: "",
			desc:     "should return empty string when only invalid chars remain",
		},
		{
			name:     "only dots and underscores",
			input:    "...__...",
			expected: "",
			desc:     "should return empty string for trimmed invalid sequences",
		},
		{
			name:     "very long URL",
			input:    "https://example.com/" + strings.Repeat("a", 150),
			expected: "", // Not used, checked separately
			desc:     "should truncate to 100 characters",
		},
		{
			name:     "URL with query parameters",
			input:    "https://example.com/api?param=value&other=test",
			expected: "https___example.com_api_param_value_other_test",
			desc:     "should replace special query characters",
		},
		{
			name:     "URL with fragment",
			input:    "https://example.com/page#section",
			expected: "https___example.com_page_section",
			desc:     "should replace hash with underscore",
		},
		{
			name:     "unicode characters",
			input:    "https://例え.jp/テスト",
			expected: "https_____.jp____",
			desc:     "should replace unicode characters with underscores",
		},
		{
			name:     "alphanumeric with dashes and dots",
			input:    "test-file.2024.log",
			expected: "test-file.2024.log",
			desc:     "should preserve valid filename characters",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			desc:     "should return empty string for empty input",
		},
		{
			name:     "whitespace only",
			input:    "   \t\n  ",
			expected: "",
			desc:     "should return empty string for whitespace only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeForFilename(tt.input)

			// For the long URL test, we need to check length separately
			if tt.name == "very long URL" {
				assert.LessOrEqual(t, len(result), 100, "result should be at most 100 characters")
				assert.True(t, len(result) > 0, "result should not be empty for long URL")
			} else {
				assert.Equal(t, tt.expected, result, tt.desc)
			}

			// Additional safety checks for non-empty results
			if result != "" {
				assert.NotContains(t, result, "..", "result should not contain .. sequences")
				assert.NotContains(t, result, "/", "result should not contain forward slashes")
				assert.NotContains(t, result, "\\", "result should not contain backslashes")
				assert.NotContains(t, result, ":", "result should not contain colons")
				assert.LessOrEqual(t, len(result), 100, "result should not exceed 100 characters")
			}
		})
	}
}

// TestSanitizeForFilename_EdgeCases tests additional edge cases
func TestSanitizeForFilename_EdgeCases(t *testing.T) {
	t.Run("embedded nulls with valid chars", func(t *testing.T) {
		input := "valid\x00chars\x00here"
		result := sanitizeForFilename(input)
		assert.NotEmpty(t, result, "should not return empty for input with some valid chars")
		assert.NotContains(t, result, "\x00", "should not contain null bytes")
	})

	t.Run("path traversal in middle of valid name", func(t *testing.T) {
		input := "start/../end"
		result := sanitizeForFilename(input)
		assert.NotEmpty(t, result, "should not return empty")
		assert.NotContains(t, result, "..", "should not contain .. sequences")
		assert.Equal(t, "start___end", result, "should sanitize .. to underscores")
	})

	t.Run("repeated separators", func(t *testing.T) {
		input := "test////file"
		result := sanitizeForFilename(input)
		assert.Equal(t, "test____file", result, "should replace each separator")
	})

	t.Run("boundary length test", func(t *testing.T) {
		// Create a string that's exactly 100 characters of valid chars
		input := strings.Repeat("a", 100)
		result := sanitizeForFilename(input)
		assert.Equal(t, 100, len(result), "should preserve 100 character limit")
	})
}
