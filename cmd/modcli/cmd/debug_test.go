package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestProject creates a temporary test project structure for testing
func createTestProject(t testing.TB) string {
	tmpDir, err := os.MkdirTemp("", "modcli-test-*")
	require.NoError(t, err)

	// Create a simple module structure
	moduleDir := filepath.Join(tmpDir, "testmodule")
	err = os.MkdirAll(moduleDir, 0755)
	require.NoError(t, err)

	// Create a test module file
	moduleContent := `package testmodule

import (
	"github.com/GoCodeAlone/modular"
	"reflect"
)

const ServiceName = "test.service"

type Module struct {
	service *TestService
}

type TestService struct{}

type TestInterface interface {
	DoSomething() error
}

func (m *Module) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Test service for unit testing",
			Instance:    m.service,
		},
	}
}

func (m *Module) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "database.connection",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*TestInterface)(nil)).Elem(),
		},
	}
}

type Config struct {
	Host string ` + "`yaml:\"host\" default:\"localhost\" desc:\"Server host\"`" + `
	Port int    ` + "`yaml:\"port\" required:\"true\" desc:\"Server port\"`" + `
}
`

	err = os.WriteFile(filepath.Join(moduleDir, "module.go"), []byte(moduleContent), 0644)
	require.NoError(t, err)

	return tmpDir
}

func TestDebugServicesCommand(t *testing.T) {
	tmpDir := createTestProject(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name: "basic services analysis",
			args: []string{"--path", tmpDir},
			expected: []string{
				"🔍 Inspecting Service Registrations",
				"test.service: TestmoduleModule",
				"Test service for unit testing",
				"database.connection: TestmoduleModule",
			},
		},
		{
			name: "verbose output",
			args: []string{"--path", tmpDir, "--verbose"},
			expected: []string{
				"🔍 Inspecting Service Registrations",
				"test.service: TestmoduleModule",
				"module.go",
			},
		},
		{
			name: "interface compatibility",
			args: []string{"--path", tmpDir, "--interfaces"},
			expected: []string{
				"🔍 Inspecting Service Registrations",
				"🔬 Interface Compatibility Checks",
				"database.connection required by TestmoduleModule is NOT provided",
			},
		},
		{
			name: "dependency graph",
			args: []string{"--path", tmpDir, "--graph"},
			expected: []string{
				"🔍 Inspecting Service Registrations",
				"🔗 Dynamic Dependency Graph",
				"TestmoduleModule",
				"├── Provides:",
				"└── Requires:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDebugServicesCommand()

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			assert.NoError(t, err)

			output := buf.String()
			for _, expected := range tt.expected {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}
		})
	}
}

func TestDebugConfigCommand(t *testing.T) {
	tmpDir := createTestProject(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name: "basic config analysis",
			args: []string{"--path", tmpDir},
			expected: []string{
				"🔍 Analyzing Module Configurations",
				"📦 Config",
				"Host (string)",
				"Port (int)",
			},
		},
		{
			name: "config validation",
			args: []string{"--path", tmpDir, "--validate"},
			expected: []string{
				"📝 Symbol Legend:",
				"⚠️  Required field",
				"Port (int)",
				"required field(s) need validation",
			},
		},
		{
			name: "show defaults",
			args: []string{"--path", tmpDir, "--show-defaults"},
			expected: []string{
				"Host (string)",
				"[default: localhost]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDebugConfigCommand()

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			assert.NoError(t, err)

			output := buf.String()
			for _, expected := range tt.expected {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}
		})
	}
}

func TestDebugDependenciesCommand(t *testing.T) {
	tmpDir := createTestProject(t)
	defer os.RemoveAll(tmpDir)

	cmd := NewDebugDependenciesCommand()

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--path", tmpDir})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	expectedStrings := []string{
		"🔍 Debugging Module Dependencies",
		"Complete Analysis Template:",
		"Common Debugging Scenarios:",
	}

	for _, expected := range expectedStrings {
		assert.Contains(t, output, expected)
	}
}

func TestDebugConfigTreeStructure(t *testing.T) {
	tmpDir := createTestProject(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name           string
		args           []string
		expectedFormat []string // Specific formatting patterns to check
		notExpected    []string // Things that shouldn't appear
	}{
		{
			name: "tree structure ends properly",
			args: []string{"--path", tmpDir},
			expectedFormat: []string{
				"📦 Config",
				"│  ├── Host (string)",
				"│  └── ⚠️  Port (int)", // Last item should use └── not ├──
			},
			notExpected: []string{
				"│  ├── ⚠️  Port (int)", // Last item shouldn't use ├──
				"│\n\n",                 // No dangling vertical lines
			},
		},
		{
			name: "validation tree structure",
			args: []string{"--path", tmpDir, "--validate"},
			expectedFormat: []string{
				"📦 Config",
				"│  ├── Host (string)",
				"│  ├── ⚠️  Port (int)",
				"│  └── ⚠️  1 required field(s) need validation", // Validation line should be last
			},
			notExpected: []string{
				"│  └── ⚠️  Port (int)", // Port shouldn't be last when validation is shown
			},
		},
		{
			name: "defaults tree structure",
			args: []string{"--path", tmpDir, "--show-defaults"},
			expectedFormat: []string{
				"📦 Config",
				"│  ├── Host (string) [default: localhost]",
				"│  └── ⚠️  Port (int)", // Last field should use └──
			},
			notExpected: []string{
				"│  ├── ⚠️  Port (int)", // Last item shouldn't use ├──
			},
		},
		{
			name: "symbol legend present",
			args: []string{"--path", tmpDir, "--validate"},
			expectedFormat: []string{
				"📝 Symbol Legend:",
				"⚠️  Required field (must be configured)",
				"✅ Optional field or has default value",
				"❌ Validation issue found",
			},
			notExpected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDebugConfigCommand()

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			assert.NoError(t, err)

			output := buf.String()

			// Check expected formatting patterns
			for _, expected := range tt.expectedFormat {
				assert.Contains(t, output, expected, "Expected tree structure pattern: %s", expected)
			}

			// Check things that shouldn't be present
			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, output, notExpected, "Should not contain improper formatting: %s", notExpected)
			}
		})
	}
}

func TestDebugServicesDependencyGraph(t *testing.T) {
	tmpDir := createTestProject(t)
	defer os.RemoveAll(tmpDir)

	cmd := NewDebugServicesCommand()

	// Test with dependency graph
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--graph"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()

	// Verify proper tree structure in dependency graph
	expectedTreeStructure := []string{
		"TestmoduleModule",
		"├── Provides:",
		"│   └── test.service",        // Should use └── for last item under Provides
		"└── Requires:",               // Should use └── since it's the last major section
		"    └── database.connection", // Should use └── for last requirement
	}

	for _, expected := range expectedTreeStructure {
		assert.Contains(t, output, expected, "Expected dependency graph tree structure: %s", expected)
	}

	// Verify status symbols are present
	assert.Contains(t, output, "❌ NOT PROVIDED", "Should show unmet dependency status")
}

func TestDebugConfigValidationSummary(t *testing.T) {
	tmpDir := createTestProject(t)
	defer os.RemoveAll(tmpDir)

	cmd := NewDebugConfigCommand()

	// Test validation summary
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--validate"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()

	// Check validation summary section
	expectedSummary := []string{
		"📋 Configuration Validation Summary:",
		"⚠️  Config: 1 required field(s)",
		"💡 Ensure all required fields are properly configured before runtime.",
	}

	for _, expected := range expectedSummary {
		assert.Contains(t, output, expected, "Expected validation summary: %s", expected)
	}
}

func TestDebugConfigOutputVisualization(t *testing.T) {
	tmpDir := createTestProject(t)
	defer os.RemoveAll(tmpDir)

	cmd := NewDebugConfigCommand()

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--path", tmpDir})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()

	// Print the output for visual inspection
	t.Logf("Config debug output:\n%s", output)

	// Verify the last item uses └── instead of ├──
	lines := strings.Split(output, "\n")
	var configLines []string
	inConfig := false

	for _, line := range lines {
		if strings.Contains(line, "📦 Config") {
			inConfig = true
			continue
		}
		if inConfig && strings.HasPrefix(line, "│  ") {
			configLines = append(configLines, line)
		}
		if inConfig && line == "" {
			break
		}
	}

	if len(configLines) > 0 {
		lastLine := configLines[len(configLines)-1]
		assert.Contains(t, lastLine, "└──", "Last config field should use └── not ├──")
		t.Logf("Last config line: %s", lastLine)
	}
}

func TestDebugConfigValidationTreeStructure(t *testing.T) {
	tmpDir := createTestProject(t)
	defer os.RemoveAll(tmpDir)

	cmd := NewDebugConfigCommand()

	// Test with validation
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--validate"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("Config validation output:\n%s", output)

	// Check that validation line is the last item and uses └──
	assert.Contains(t, output, "│  └── ⚠️  1 required field(s) need validation",
		"Validation line should be last and use └──")

	// Check that Port field is NOT the last item (should use ├──)
	assert.Contains(t, output, "│  ├── ⚠️  Port (int)",
		"Port field should use ├── when validation line follows")
}
