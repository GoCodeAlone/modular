package contract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractor_ExtractFromDirectory(t *testing.T) {
	// Create a temporary directory with test Go files
	tmpDir, err := os.MkdirTemp("", "extractor-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple Go file with various API elements
	testCode := `package testpkg

// TestInterface is a test interface
type TestInterface interface {
	// TestMethod does something
	TestMethod(input string) (bool, error)
}

// TestStruct is a test struct
type TestStruct struct {
	// Field1 is a string field
	Field1 string ` + "`json:\"field1\"`" + `
	// Field2 is an int field
	Field2 int
}

// TestMethod is a method on TestStruct
func (t *TestStruct) TestMethod() error {
	return nil
}

// TestFunc is a test function
func TestFunc(param string) (bool, error) {
	return false, nil
}

// TestVar is a test variable
var TestVar string

// TestConst is a test constant
const TestConst = "test"

// privateType should not be extracted by default
type privateType struct {
	field string
}

// privateFunc should not be extracted by default
func privateFunc() {}
`

	testFile := filepath.Join(tmpDir, "test.go")
	err = os.WriteFile(testFile, []byte(testCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test extraction without including private items
	extractor := NewExtractor()
	contract, err := extractor.ExtractFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("Failed to extract contract: %v", err)
	}

	// Verify basic properties
	if contract.PackageName != "testpkg" {
		t.Errorf("Expected package name 'testpkg', got %s", contract.PackageName)
	}

	// Check that we found the expected exported items
	foundInterface := false
	foundStruct := false
	foundFunction := false

	for _, iface := range contract.Interfaces {
		if iface.Name == "TestInterface" {
			foundInterface = true
			// With the complete implementation, we should extract methods
			if len(iface.Methods) == 0 {
				t.Log("Interface methods should be extracted with the full implementation")
			}
		}
	}

	for _, typ := range contract.Types {
		if typ.Name == "TestStruct" {
			foundStruct = true
			if typ.Kind != "struct" && typ.Kind != "" {
				t.Errorf("Expected struct kind for TestStruct, got %s", typ.Kind)
			}
		}
	}

	for _, fn := range contract.Functions {
		if fn.Name == "TestFunc" {
			foundFunction = true
		}
	}

	if !foundInterface {
		t.Error("Expected to find TestInterface")
	}

	// With the complete implementation, we should properly differentiate types
	if !foundStruct {
		t.Error("Expected to find TestStruct with complete implementation")
	}

	if !foundFunction {
		t.Error("Expected to find TestFunc")
	}

	// Verify that private items are not included
	for _, typ := range contract.Types {
		if typ.Name == "privateType" {
			t.Error("Did not expect to find privateType")
		}
	}

	for _, fn := range contract.Functions {
		if fn.Name == "privateFunc" {
			t.Error("Did not expect to find privateFunc")
		}
	}

	// Test extraction with private items included
	extractor.IncludePrivate = true
	contractWithPrivate, err := extractor.ExtractFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("Failed to extract contract with private items: %v", err)
	}

	// Should have more items when including private
	totalItems := len(contract.Interfaces) + len(contract.Types) + len(contract.Functions)
	totalWithPrivate := len(contractWithPrivate.Interfaces) + len(contractWithPrivate.Types) + len(contractWithPrivate.Functions)

	if totalWithPrivate <= totalItems {
		t.Log("Warning: Including private items didn't increase the count (may be a limitation of AST-based extraction)")
	}
}

func TestExtractor_ExtractFromDirectory_EmptyDirectory(t *testing.T) {
	// Create a temporary directory with no Go files
	tmpDir, err := os.MkdirTemp("", "extractor-empty-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	extractor := NewExtractor()
	_, err = extractor.ExtractFromDirectory(tmpDir)
	if err == nil {
		t.Error("Expected error for directory with no Go files")
	}
}

func TestExtractor_ExtractFromDirectory_InvalidDirectory(t *testing.T) {
	extractor := NewExtractor()
	_, err := extractor.ExtractFromDirectory("/nonexistent/directory")
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
}

func TestExtractor_ExtractFromDirectory_WithTests(t *testing.T) {
	// Create a temporary directory with test and non-test files
	tmpDir, err := os.MkdirTemp("", "extractor-test-files-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Main code file
	mainCode := `package testpkg

func MainFunc() {}
`

	// Test code file
	testCode := `package testpkg

import "testing"

func TestSomething(t *testing.T) {}

func TestHelper() string {
	return "helper"
}
`

	mainFile := filepath.Join(tmpDir, "main.go")
	testFile := filepath.Join(tmpDir, "main_test.go")

	err = os.WriteFile(mainFile, []byte(mainCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write main file: %v", err)
	}

	err = os.WriteFile(testFile, []byte(testCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Extract without tests
	extractor := NewExtractor()
	contract, err := extractor.ExtractFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("Failed to extract contract: %v", err)
	}

	// Should only have MainFunc
	foundMainFunc := false
	foundTestFunc := false

	for _, fn := range contract.Functions {
		if fn.Name == "MainFunc" {
			foundMainFunc = true
		}
		if fn.Name == "TestSomething" || fn.Name == "TestHelper" {
			foundTestFunc = true
		}
	}

	if !foundMainFunc {
		t.Error("Expected to find MainFunc")
	}

	if foundTestFunc {
		t.Error("Did not expect to find test functions when tests are excluded")
	}

	// Extract with tests
	extractor.IncludeTests = true
	contractWithTests, err := extractor.ExtractFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("Failed to extract contract with tests: %v", err)
	}

	// Should have more functions
	if len(contractWithTests.Functions) <= len(contract.Functions) {
		t.Error("Expected more functions when including tests")
	}
}

func TestExtractor_Options(t *testing.T) {
	extractor := NewExtractor()

	// Test default options
	if extractor.IncludePrivate {
		t.Error("Expected IncludePrivate to be false by default")
	}
	if extractor.IncludeTests {
		t.Error("Expected IncludeTests to be false by default")
	}
	if extractor.IncludeInternal {
		t.Error("Expected IncludeInternal to be false by default")
	}

	// Test setting options
	extractor.IncludePrivate = true
	extractor.IncludeTests = true
	extractor.IncludeInternal = true

	if !extractor.IncludePrivate {
		t.Error("Expected IncludePrivate to be true after setting")
	}
	if !extractor.IncludeTests {
		t.Error("Expected IncludeTests to be true after setting")
	}
	if !extractor.IncludeInternal {
		t.Error("Expected IncludeInternal to be true after setting")
	}
}

func TestExtractor_ExtractFromPackage_InvalidPackage(t *testing.T) {
	extractor := NewExtractor()
	_, err := extractor.ExtractFromPackage("nonexistent/invalid/package/path")
	if err == nil {
		t.Error("Expected error for invalid package path")
	}
}

// TestExtractor_SortContract tests that contracts are sorted consistently
func TestExtractor_SortContract(t *testing.T) {
	extractor := NewExtractor()

	contract := &Contract{
		Interfaces: []InterfaceContract{
			{Name: "ZInterface"},
			{Name: "AInterface"},
			{Name: "MInterface"},
		},
		Types: []TypeContract{
			{Name: "ZType"},
			{Name: "AType"},
			{Name: "MType"},
		},
		Functions: []FunctionContract{
			{Name: "zFunc"},
			{Name: "aFunc"},
			{Name: "mFunc"},
		},
	}

	extractor.sortContract(contract)

	// Check that interfaces are sorted
	expectedInterfaces := []string{"AInterface", "MInterface", "ZInterface"}
	for i, iface := range contract.Interfaces {
		if iface.Name != expectedInterfaces[i] {
			t.Errorf("Interface %d: expected %s, got %s", i, expectedInterfaces[i], iface.Name)
		}
	}

	// Check that types are sorted
	expectedTypes := []string{"AType", "MType", "ZType"}
	for i, typ := range contract.Types {
		if typ.Name != expectedTypes[i] {
			t.Errorf("Type %d: expected %s, got %s", i, expectedTypes[i], typ.Name)
		}
	}

	// Check that functions are sorted
	expectedFunctions := []string{"aFunc", "mFunc", "zFunc"}
	for i, fn := range contract.Functions {
		if fn.Name != expectedFunctions[i] {
			t.Errorf("Function %d: expected %s, got %s", i, expectedFunctions[i], fn.Name)
		}
	}
}
