package contract

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestContract_SaveAndLoad(t *testing.T) {
	// Create a test contract
	testContract := &Contract{
		PackageName: "testpkg",
		ModulePath:  "github.com/test/pkg",
		Version:     "v1.0.0",
		Timestamp:   time.Now().Truncate(time.Second), // Truncate for comparison
		Interfaces: []InterfaceContract{
			{
				Name:       "TestInterface",
				Package:    "testpkg",
				DocComment: "Test interface documentation",
				Methods: []MethodContract{
					{
						Name: "TestMethod",
						Parameters: []ParameterInfo{
							{Name: "input", Type: "string"},
						},
						Results: []ParameterInfo{
							{Type: "error"},
						},
					},
				},
			},
		},
		Types: []TypeContract{
			{
				Name:    "TestStruct",
				Package: "testpkg",
				Kind:    "struct",
				Fields: []FieldContract{
					{
						Name: "Field1",
						Type: "string",
						Tag:  `json:"field1"`,
					},
					{
						Name: "Field2",
						Type: "int",
					},
				},
			},
		},
		Functions: []FunctionContract{
			{
				Name:    "TestFunc",
				Package: "testpkg",
				Parameters: []ParameterInfo{
					{Name: "param", Type: "string"},
				},
				Results: []ParameterInfo{
					{Type: "bool"},
					{Type: "error"},
				},
			},
		},
		Variables: []VariableContract{
			{
				Name:    "TestVar",
				Package: "testpkg",
				Type:    "string",
			},
		},
		Constants: []ConstantContract{
			{
				Name:    "TestConst",
				Package: "testpkg",
				Type:    "string",
				Value:   `"test"`,
			},
		},
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "contract-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Test saving
	err = testContract.SaveToFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to save contract: %v", err)
	}

	// Test loading
	loaded, err := LoadFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load contract: %v", err)
	}

	// Compare contracts
	if loaded.PackageName != testContract.PackageName {
		t.Errorf("Package name mismatch: got %s, want %s", loaded.PackageName, testContract.PackageName)
	}

	if loaded.ModulePath != testContract.ModulePath {
		t.Errorf("Module path mismatch: got %s, want %s", loaded.ModulePath, testContract.ModulePath)
	}

	if loaded.Version != testContract.Version {
		t.Errorf("Version mismatch: got %s, want %s", loaded.Version, testContract.Version)
	}

	// Check that timestamp is close (within a few seconds)
	timeDiff := loaded.Timestamp.Sub(testContract.Timestamp)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > time.Minute {
		t.Errorf("Timestamp difference too large: %v", timeDiff)
	}

	// Check interfaces
	if len(loaded.Interfaces) != len(testContract.Interfaces) {
		t.Errorf("Interface count mismatch: got %d, want %d", len(loaded.Interfaces), len(testContract.Interfaces))
	} else if len(loaded.Interfaces) > 0 {
		loadedIface := loaded.Interfaces[0]
		originalIface := testContract.Interfaces[0]

		if loadedIface.Name != originalIface.Name {
			t.Errorf("Interface name mismatch: got %s, want %s", loadedIface.Name, originalIface.Name)
		}

		if len(loadedIface.Methods) != len(originalIface.Methods) {
			t.Errorf("Method count mismatch: got %d, want %d", len(loadedIface.Methods), len(originalIface.Methods))
		}
	}

	// Check types
	if len(loaded.Types) != len(testContract.Types) {
		t.Errorf("Type count mismatch: got %d, want %d", len(loaded.Types), len(testContract.Types))
	} else if len(loaded.Types) > 0 {
		loadedType := loaded.Types[0]
		originalType := testContract.Types[0]

		if loadedType.Name != originalType.Name {
			t.Errorf("Type name mismatch: got %s, want %s", loadedType.Name, originalType.Name)
		}

		if loadedType.Kind != originalType.Kind {
			t.Errorf("Type kind mismatch: got %s, want %s", loadedType.Kind, originalType.Kind)
		}

		if len(loadedType.Fields) != len(originalType.Fields) {
			t.Errorf("Field count mismatch: got %d, want %d", len(loadedType.Fields), len(originalType.Fields))
		}
	}

	// Check functions, variables, constants counts
	if len(loaded.Functions) != len(testContract.Functions) {
		t.Errorf("Function count mismatch: got %d, want %d", len(loaded.Functions), len(testContract.Functions))
	}

	if len(loaded.Variables) != len(testContract.Variables) {
		t.Errorf("Variable count mismatch: got %d, want %d", len(loaded.Variables), len(testContract.Variables))
	}

	if len(loaded.Constants) != len(testContract.Constants) {
		t.Errorf("Constant count mismatch: got %d, want %d", len(loaded.Constants), len(testContract.Constants))
	}
}

func TestContract_MarshalJSON(t *testing.T) {
	contract := &Contract{
		PackageName: "testpkg",
		Version:     "v1.0.0",
		Timestamp:   time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("Failed to marshal contract: %v", err)
	}

	var unmarshaled Contract
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal contract: %v", err)
	}

	if unmarshaled.PackageName != contract.PackageName {
		t.Errorf("Package name mismatch after JSON round-trip: got %s, want %s",
			unmarshaled.PackageName, contract.PackageName)
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("nonexistent-file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	// Create temporary file with invalid JSON
	tmpFile, err := os.CreateTemp("", "invalid-contract-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("invalid json content")
	if err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}
	tmpFile.Close()

	_, err = LoadFromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
