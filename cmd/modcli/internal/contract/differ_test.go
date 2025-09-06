package contract

import (
	"os"
	"testing"
	"time"
)

func TestDiffer_Compare(t *testing.T) {
	// Create old contract
	oldContract := &Contract{
		PackageName: "testpkg",
		Version:     "v1.0.0",
		Timestamp:   time.Now(),
		Interfaces: []InterfaceContract{
			{
				Name:    "OldInterface",
				Package: "testpkg",
				Methods: []MethodContract{
					{Name: "ExistingMethod"},
					{Name: "ToBeRemovedMethod"},
				},
			},
		},
		Types: []TypeContract{
			{
				Name:    "ExistingType",
				Package: "testpkg",
				Kind:    "struct",
				Fields: []FieldContract{
					{Name: "ExistingField", Type: "string"},
					{Name: "ToBeRemovedField", Type: "int"},
				},
			},
		},
		Functions: []FunctionContract{
			{
				Name:    "ExistingFunc",
				Package: "testpkg",
				Parameters: []ParameterInfo{
					{Name: "param", Type: "string"},
				},
			},
		},
		Variables: []VariableContract{
			{Name: "ExistingVar", Package: "testpkg", Type: "string"},
		},
		Constants: []ConstantContract{
			{Name: "ExistingConst", Package: "testpkg", Type: "int", Value: "42"},
		},
	}

	// Create new contract with changes
	newContract := &Contract{
		PackageName: "testpkg",
		Version:     "v2.0.0",
		Timestamp:   time.Now(),
		Interfaces: []InterfaceContract{
			{
				Name:    "OldInterface",
				Package: "testpkg",
				Methods: []MethodContract{
					{Name: "ExistingMethod"},
					{Name: "NewMethod"}, // Added
					// ToBeRemovedMethod removed - breaking change
				},
			},
			{
				Name:    "NewInterface", // Added
				Package: "testpkg",
				Methods: []MethodContract{
					{Name: "SomeMethod"},
				},
			},
		},
		Types: []TypeContract{
			{
				Name:    "ExistingType",
				Package: "testpkg",
				Kind:    "struct",
				Fields: []FieldContract{
					{Name: "ExistingField", Type: "string"},
					{Name: "NewField", Type: "bool"}, // Added
					// ToBeRemovedField removed - breaking change
				},
			},
			{
				Name:    "NewType", // Added
				Package: "testpkg",
				Kind:    "struct",
			},
		},
		Functions: []FunctionContract{
			{
				Name:    "ExistingFunc",
				Package: "testpkg",
				Parameters: []ParameterInfo{
					{Name: "param", Type: "int"}, // Changed type - breaking change
				},
			},
			{
				Name:    "NewFunc", // Added
				Package: "testpkg",
			},
		},
		Variables: []VariableContract{
			{Name: "ExistingVar", Package: "testpkg", Type: "int"}, // Changed type - breaking change
			{Name: "NewVar", Package: "testpkg", Type: "string"},   // Added
		},
		Constants: []ConstantContract{
			{Name: "ExistingConst", Package: "testpkg", Type: "int", Value: "100"},  // Changed value
			{Name: "NewConst", Package: "testpkg", Type: "string", Value: `"test"`}, // Added
		},
	}

	differ := NewDiffer()
	diff, err := differ.Compare(oldContract, newContract)
	if err != nil {
		t.Fatalf("Failed to compare contracts: %v", err)
	}

	// Check summary
	if !diff.Summary.HasBreakingChanges {
		t.Error("Expected breaking changes to be detected")
	}

	expectedBreakingChanges := 4 // removed method, removed field, changed function signature, changed variable type
	if diff.Summary.TotalBreakingChanges != expectedBreakingChanges {
		t.Errorf("Expected %d breaking changes, got %d", expectedBreakingChanges, diff.Summary.TotalBreakingChanges)
	}

	expectedAdditions := 5 // new interface, new method, new field, new type, new function, new variable, new constant
	if diff.Summary.TotalAdditions < expectedAdditions {
		t.Errorf("Expected at least %d additions, got %d", expectedAdditions, diff.Summary.TotalAdditions)
	}

	// Check that we have specific breaking changes
	foundRemovedMethod := false
	foundRemovedField := false
	foundChangedFuncSignature := false
	foundChangedVarType := false

	for _, change := range diff.BreakingChanges {
		switch change.Type {
		case "removed_method":
			if change.Item == "OldInterface.ToBeRemovedMethod" {
				foundRemovedMethod = true
			}
		case "removed_field":
			if change.Item == "ExistingType.ToBeRemovedField" {
				foundRemovedField = true
			}
		case "changed_function_signature":
			if change.Item == "ExistingFunc" {
				foundChangedFuncSignature = true
			}
		case "changed_variable_type":
			if change.Item == "ExistingVar" {
				foundChangedVarType = true
			}
		}
	}

	if !foundRemovedMethod {
		t.Error("Expected to find removed method breaking change")
	}
	if !foundRemovedField {
		t.Error("Expected to find removed field breaking change")
	}
	if !foundChangedFuncSignature {
		t.Error("Expected to find changed function signature breaking change")
	}
	if !foundChangedVarType {
		t.Error("Expected to find changed variable type breaking change")
	}

	// Check for additions
	foundNewInterface := false
	foundNewMethod := false
	for _, addition := range diff.AddedItems {
		if addition.Type == "interface" && addition.Item == "NewInterface" {
			foundNewInterface = true
		}
		if addition.Type == "method" && addition.Item == "OldInterface.NewMethod" {
			foundNewMethod = true
		}
	}

	if !foundNewInterface {
		t.Error("Expected to find new interface addition")
	}
	if !foundNewMethod {
		t.Error("Expected to find new method addition")
	}
}

func TestDiffer_Compare_NilContracts(t *testing.T) {
	differ := NewDiffer()

	_, err := differ.Compare(nil, &Contract{})
	if err == nil {
		t.Error("Expected error for nil old contract")
	}

	_, err = differ.Compare(&Contract{}, nil)
	if err == nil {
		t.Error("Expected error for nil new contract")
	}
}

func TestDiffer_MethodSignaturesEqual(t *testing.T) {
	differ := NewDiffer()

	// Same methods
	method1 := MethodContract{
		Name: "TestMethod",
		Parameters: []ParameterInfo{
			{Name: "param1", Type: "string"},
			{Name: "param2", Type: "int"},
		},
		Results: []ParameterInfo{
			{Type: "bool"},
			{Type: "error"},
		},
	}

	method2 := MethodContract{
		Name: "TestMethod",
		Parameters: []ParameterInfo{
			{Name: "param1", Type: "string"},
			{Name: "param2", Type: "int"},
		},
		Results: []ParameterInfo{
			{Type: "bool"},
			{Type: "error"},
		},
	}

	if !differ.methodSignaturesEqual(method1, method2) {
		t.Error("Expected identical methods to be equal")
	}

	// Different parameter types
	method3 := MethodContract{
		Name: "TestMethod",
		Parameters: []ParameterInfo{
			{Name: "param1", Type: "int"}, // Changed type
			{Name: "param2", Type: "int"},
		},
		Results: []ParameterInfo{
			{Type: "bool"},
			{Type: "error"},
		},
	}

	if differ.methodSignaturesEqual(method1, method3) {
		t.Error("Expected methods with different parameter types to be different")
	}

	// Different number of parameters
	method4 := MethodContract{
		Name: "TestMethod",
		Parameters: []ParameterInfo{
			{Name: "param1", Type: "string"},
			// Missing param2
		},
		Results: []ParameterInfo{
			{Type: "bool"},
			{Type: "error"},
		},
	}

	if differ.methodSignaturesEqual(method1, method4) {
		t.Error("Expected methods with different parameter counts to be different")
	}

	// Different return types
	method5 := MethodContract{
		Name: "TestMethod",
		Parameters: []ParameterInfo{
			{Name: "param1", Type: "string"},
			{Name: "param2", Type: "int"},
		},
		Results: []ParameterInfo{
			{Type: "string"}, // Changed type
			{Type: "error"},
		},
	}

	if differ.methodSignaturesEqual(method1, method5) {
		t.Error("Expected methods with different return types to be different")
	}
}

func TestDiffer_SaveAndLoadDiff(t *testing.T) {
	diff := &ContractDiff{
		PackageName: "testpkg",
		OldVersion:  "v1.0.0",
		NewVersion:  "v2.0.0",
		BreakingChanges: []BreakingChange{
			{
				Type:        "removed_method",
				Item:        "Interface.Method",
				Description: "Method was removed",
			},
		},
		AddedItems: []AddedItem{
			{
				Type:        "interface",
				Item:        "NewInterface",
				Description: "New interface was added",
			},
		},
		Summary: DiffSummary{
			TotalBreakingChanges: 1,
			TotalAdditions:       1,
			HasBreakingChanges:   true,
		},
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "diff-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Test saving
	err = diff.SaveToFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to save diff: %v", err)
	}

	// Test loading
	loaded, err := LoadDiffFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load diff: %v", err)
	}

	// Compare diffs
	if loaded.PackageName != diff.PackageName {
		t.Errorf("Package name mismatch: got %s, want %s", loaded.PackageName, diff.PackageName)
	}

	if loaded.Summary.HasBreakingChanges != diff.Summary.HasBreakingChanges {
		t.Errorf("Breaking changes flag mismatch: got %t, want %t",
			loaded.Summary.HasBreakingChanges, diff.Summary.HasBreakingChanges)
	}

	if len(loaded.BreakingChanges) != len(diff.BreakingChanges) {
		t.Errorf("Breaking changes count mismatch: got %d, want %d",
			len(loaded.BreakingChanges), len(diff.BreakingChanges))
	}

	if len(loaded.AddedItems) != len(diff.AddedItems) {
		t.Errorf("Added items count mismatch: got %d, want %d",
			len(loaded.AddedItems), len(diff.AddedItems))
	}
}

func TestDiffer_formatDiff(t *testing.T) {
	testDiff := &ContractDiff{
		PackageName: "testpkg",
		BreakingChanges: []BreakingChange{
			{
				Type:        "removed_method",
				Item:        "TestInterface.TestMethod",
				Description: "Method TestMethod was removed from interface TestInterface",
			},
		},
		AddedItems: []AddedItem{
			{
				Type:        "interface",
				Item:        "NewInterface",
				Description: "New interface NewInterface was added",
			},
		},
		Summary: DiffSummary{
			TotalBreakingChanges: 1,
			TotalAdditions:       1,
			HasBreakingChanges:   true,
		},
	}

	// Test that the diff has the expected properties
	if testDiff.PackageName != "testpkg" {
		t.Errorf("Expected package name 'testpkg', got %s", testDiff.PackageName)
	}

	differ := NewDiffer()

	// Test method signature formatting
	method := MethodContract{
		Name: "TestMethod",
		Parameters: []ParameterInfo{
			{Name: "param1", Type: "string"},
			{Name: "param2", Type: "int"},
		},
		Results: []ParameterInfo{
			{Type: "bool"},
			{Type: "error"},
		},
	}

	signature := differ.methodSignature(method)
	expected := "TestMethod (param1 string, param2 int) (bool, error)"
	if signature != expected {
		t.Errorf("Method signature format mismatch: got %s, want %s", signature, expected)
	}

	// Test function signature formatting
	fn := FunctionContract{
		Name: "TestFunc",
		Parameters: []ParameterInfo{
			{Name: "input", Type: "string"},
		},
		Results: []ParameterInfo{
			{Type: "error"},
		},
	}

	fnSig := differ.functionSignature(fn)
	expectedFn := "TestFunc (input string) (error)"
	if fnSig != expectedFn {
		t.Errorf("Function signature format mismatch: got %s, want %s", fnSig, expectedFn)
	}
}
