
package modular

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigDiff(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_have_config_diff_type_defined",
			testFunc: func(t *testing.T) {
				// Test that ConfigDiff type exists
				var diff ConfigDiff
				assert.NotNil(t, diff, "ConfigDiff type should be defined")
			},
		},
		{
			name: "should_define_changed_fields",
			testFunc: func(t *testing.T) {
				// Test that ConfigDiff has Changed field
				diff := ConfigDiff{
					Changed: map[string]ConfigFieldChange{
						"database.host": {
							OldValue: "localhost",
							NewValue: "db.example.com",
							FieldPath: "database.host",
						},
					},
				}
				assert.Len(t, diff.Changed, 1, "ConfigDiff should have Changed field")
			},
		},
		{
			name: "should_define_added_fields",
			testFunc: func(t *testing.T) {
				// Test that ConfigDiff has Added field
				diff := ConfigDiff{
					Added: map[string]interface{}{
						"cache.ttl": "5m",
					},
				}
				assert.Len(t, diff.Added, 1, "ConfigDiff should have Added field")
			},
		},
		{
			name: "should_define_removed_fields",
			testFunc: func(t *testing.T) {
				// Test that ConfigDiff has Removed field
				diff := ConfigDiff{
					Removed: map[string]interface{}{
						"deprecated.option": "old_value",
					},
				}
				assert.Len(t, diff.Removed, 1, "ConfigDiff should have Removed field")
			},
		},
		{
			name: "should_define_timestamp_field",
			testFunc: func(t *testing.T) {
				// Test that ConfigDiff has Timestamp field
				timestamp := time.Now()
				diff := ConfigDiff{
					Timestamp: timestamp,
				}
				assert.Equal(t, timestamp, diff.Timestamp, "ConfigDiff should have Timestamp field")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestConfigFieldChange(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_have_config_field_change_type",
			testFunc: func(t *testing.T) {
				// Test that ConfigFieldChange type exists with all fields
				change := ConfigFieldChange{
					FieldPath: "server.port",
					OldValue:  8080,
					NewValue:  9090,
					ChangeType: ChangeTypeModified,
				}
				assert.Equal(t, "server.port", change.FieldPath, "ConfigFieldChange should have FieldPath")
				assert.Equal(t, 8080, change.OldValue, "ConfigFieldChange should have OldValue")
				assert.Equal(t, 9090, change.NewValue, "ConfigFieldChange should have NewValue")
				assert.Equal(t, ChangeTypeModified, change.ChangeType, "ConfigFieldChange should have ChangeType")
			},
		},
		{
			name: "should_support_sensitive_field_marking",
			testFunc: func(t *testing.T) {
				// Test that ConfigFieldChange can mark sensitive fields
				change := ConfigFieldChange{
					FieldPath:   "database.password",
					OldValue:    "old_secret",
					NewValue:    "new_secret",
					ChangeType:  ChangeTypeModified,
					IsSensitive: true,
				}
				assert.True(t, change.IsSensitive, "ConfigFieldChange should support IsSensitive flag")
			},
		},
		{
			name: "should_support_validation_info",
			testFunc: func(t *testing.T) {
				// Test that ConfigFieldChange can include validation information
				change := ConfigFieldChange{
					FieldPath:        "server.timeout",
					OldValue:         "30s",
					NewValue:         "60s",
					ChangeType:       ChangeTypeModified,
					ValidationResult: &ValidationResult{IsValid: true, Message: "Valid duration"},
				}
				assert.NotNil(t, change.ValidationResult, "ConfigFieldChange should support ValidationResult")
				assert.True(t, change.ValidationResult.IsValid, "ValidationResult should have IsValid field")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestChangeType(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_change_type_constants",
			testFunc: func(t *testing.T) {
				// Test that ChangeType constants are defined
				assert.Equal(t, "added", string(ChangeTypeAdded), "ChangeTypeAdded should be 'added'")
				assert.Equal(t, "modified", string(ChangeTypeModified), "ChangeTypeModified should be 'modified'")
				assert.Equal(t, "removed", string(ChangeTypeRemoved), "ChangeTypeRemoved should be 'removed'")
			},
		},
		{
			name: "should_support_string_conversion",
			testFunc: func(t *testing.T) {
				// Test that ChangeType can be converted to string
				changeType := ChangeTypeModified
				str := changeType.String()
				assert.Equal(t, "modified", str, "ChangeType should convert to string")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestConfigDiffGeneration(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "should_generate_diff_between_config_structs",
			description: "ConfigDiff should be generated by comparing two configuration objects",
			testFunc: func(t *testing.T) {
				// Test config structures
				oldConfig := testConfig{
					DatabaseHost: "localhost",
					ServerPort:   8080,
					CacheTTL:     "5m",
				}

				newConfig := testConfig{
					DatabaseHost: "db.example.com",
					ServerPort:   9090,
					CacheTTL:     "10m",
				}

				diff, err := GenerateConfigDiff(oldConfig, newConfig)
				assert.NoError(t, err, "GenerateConfigDiff should succeed")
				assert.NotNil(t, diff, "GenerateConfigDiff should return ConfigDiff")
				assert.Greater(t, len(diff.Changed), 0, "Diff should detect changed fields")
			},
		},
		{
			name:        "should_detect_added_fields",
			description: "ConfigDiff should detect newly added configuration fields",
			testFunc: func(t *testing.T) {
				oldConfig := map[string]interface{}{
					"server": map[string]interface{}{
						"port": 8080,
					},
				}

				newConfig := map[string]interface{}{
					"server": map[string]interface{}{
						"port": 8080,
						"host": "0.0.0.0", // New field
					},
					"database": map[string]interface{}{ // New section
						"host": "localhost",
					},
				}

				diff, err := GenerateConfigDiff(oldConfig, newConfig)
				assert.NoError(t, err, "GenerateConfigDiff should succeed")
				assert.Greater(t, len(diff.Added), 0, "Diff should detect added fields")
				assert.Contains(t, diff.Added, "server.host", "Should detect added server.host field")
			},
		},
		{
			name:        "should_detect_removed_fields",
			description: "ConfigDiff should detect removed configuration fields",
			testFunc: func(t *testing.T) {
				oldConfig := map[string]interface{}{
					"server": map[string]interface{}{
						"port":    8080,
						"host":    "localhost",
						"timeout": "30s", // Will be removed
					},
					"deprecated": map[string]interface{}{ // Will be removed
						"option": "value",
					},
				}

				newConfig := map[string]interface{}{
					"server": map[string]interface{}{
						"port": 8080,
						"host": "localhost",
					},
				}

				diff, err := GenerateConfigDiff(oldConfig, newConfig)
				assert.NoError(t, err, "GenerateConfigDiff should succeed")
				assert.Greater(t, len(diff.Removed), 0, "Diff should detect removed fields")
				assert.Contains(t, diff.Removed, "server.timeout", "Should detect removed timeout field")
			},
		},
		{
			name:        "should_handle_nested_struct_changes",
			description: "ConfigDiff should properly handle changes in nested configuration structures",
			testFunc: func(t *testing.T) {
				oldConfig := nestedTestConfig{
					Server: serverConfig{
						Port:    8080,
						Host:    "localhost",
						Timeout: "30s",
					},
					Database: databaseConfig{
						Host:     "localhost",
						Port:     5432,
						Username: "user",
					},
				}

				newConfig := nestedTestConfig{
					Server: serverConfig{
						Port:    9090, // Changed
						Host:    "0.0.0.0", // Changed
						Timeout: "30s",
					},
					Database: databaseConfig{
						Host:     "db.example.com", // Changed
						Port:     5432,
						Username: "admin", // Changed
					},
				}

				diff, err := GenerateConfigDiff(oldConfig, newConfig)
				assert.NoError(t, err, "GenerateConfigDiff should succeed")
				assert.Greater(t, len(diff.Changed), 0, "Should detect changes in nested structs")
				
				// Check specific field paths
				assert.Contains(t, diff.Changed, "server.port", "Should detect server.port change")
				assert.Contains(t, diff.Changed, "database.host", "Should detect database.host change")
			},
		},
		{
			name:        "should_handle_sensitive_fields",
			description: "ConfigDiff should mark sensitive fields and not expose their values",
			testFunc: func(t *testing.T) {
				oldConfig := sensitiveTestConfig{
					DatabasePassword: "old_secret",
					APIKey:          "old_api_key",
					PublicConfig:    "public_value",
				}

				newConfig := sensitiveTestConfig{
					DatabasePassword: "new_secret",
					APIKey:          "new_api_key",
					PublicConfig:    "new_public_value",
				}

				diff, err := GenerateConfigDiff(oldConfig, newConfig)
				assert.NoError(t, err, "GenerateConfigDiff should succeed")

				// Check that sensitive fields are marked appropriately
				if passwordChange, exists := diff.Changed["database_password"]; exists {
					assert.True(t, passwordChange.IsSensitive, "Password field should be marked as sensitive")
					assert.Equal(t, "[REDACTED]", passwordChange.OldValue, "Sensitive old value should be redacted")
					assert.Equal(t, "[REDACTED]", passwordChange.NewValue, "Sensitive new value should be redacted")
				}

				// Check that non-sensitive fields are not redacted
				if publicChange, exists := diff.Changed["public_config"]; exists {
					assert.False(t, publicChange.IsSensitive, "Public field should not be marked as sensitive")
					assert.NotEqual(t, "[REDACTED]", publicChange.OldValue, "Public old value should not be redacted")
					assert.NotEqual(t, "[REDACTED]", publicChange.NewValue, "Public new value should not be redacted")
				}
			},
		},
		{
			name:        "should_support_diff_options",
			description: "ConfigDiff generation should support various options for customization",
			testFunc: func(t *testing.T) {
				oldConfig := testConfig{
					DatabaseHost: "localhost",
					ServerPort:   8080,
				}

				newConfig := testConfig{
					DatabaseHost: "db.example.com",
					ServerPort:   9090,
				}

				options := ConfigDiffOptions{
					IgnoreFields:      []string{"server_port"}, // Should ignore port changes
					SensitiveFields:   []string{"database_host"}, // Treat host as sensitive
					IncludeValidation: true,
				}

				diff, err := GenerateConfigDiffWithOptions(oldConfig, newConfig, options)
				assert.NoError(t, err, "GenerateConfigDiffWithOptions should succeed")
				assert.NotContains(t, diff.Changed, "server_port", "Should ignore specified fields")
				
				if hostChange, exists := diff.Changed["database_host"]; exists {
					assert.True(t, hostChange.IsSensitive, "Should mark specified fields as sensitive")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestConfigDiffMethods(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_check_if_diff_has_changes",
			testFunc: func(t *testing.T) {
				// Test empty diff
				emptyDiff := ConfigDiff{}
				assert.False(t, emptyDiff.HasChanges(), "Empty diff should report no changes")

				// Test diff with changes
				diffWithChanges := ConfigDiff{
					Changed: map[string]ConfigFieldChange{
						"field": {FieldPath: "field", OldValue: "old", NewValue: "new"},
					},
				}
				assert.True(t, diffWithChanges.HasChanges(), "Diff with changes should report changes")
			},
		},
		{
			name: "should_get_change_summary",
			testFunc: func(t *testing.T) {
				diff := ConfigDiff{
					Changed: map[string]ConfigFieldChange{
						"field1": {},
						"field2": {},
					},
					Added:   map[string]interface{}{"field3": "value"},
					Removed: map[string]interface{}{"field4": "value"},
				}

				summary := diff.ChangeSummary()
				assert.Equal(t, 2, summary.ModifiedCount, "Should count modified fields")
				assert.Equal(t, 1, summary.AddedCount, "Should count added fields")
				assert.Equal(t, 1, summary.RemovedCount, "Should count removed fields")
				assert.Equal(t, 4, summary.TotalChanges, "Should count total changes")
			},
		},
		{
			name: "should_filter_changes_by_module",
			testFunc: func(t *testing.T) {
				diff := ConfigDiff{
					Changed: map[string]ConfigFieldChange{
						"database.host":     {},
						"database.port":     {},
						"httpserver.port":   {},
						"httpserver.timeout": {},
					},
				}

				databaseChanges := diff.FilterByPrefix("database")
				assert.Len(t, databaseChanges.Changed, 2, "Should filter database changes")
				assert.Contains(t, databaseChanges.Changed, "database.host")
				assert.Contains(t, databaseChanges.Changed, "database.port")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

// Test helper types
type testConfig struct {
	DatabaseHost string `json:"database_host"`
	ServerPort   int    `json:"server_port"`
	CacheTTL     string `json:"cache_ttl"`
}

type serverConfig struct {
	Port    int    `json:"port"`
	Host    string `json:"host"`
	Timeout string `json:"timeout"`
}

type databaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
}

type nestedTestConfig struct {
	Server   serverConfig   `json:"server"`
	Database databaseConfig `json:"database"`
}

type sensitiveTestConfig struct {
	DatabasePassword string `json:"database_password" sensitive:"true"`
	APIKey          string `json:"api_key" sensitive:"true"`
	PublicConfig    string `json:"public_config"`
}