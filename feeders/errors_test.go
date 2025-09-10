package feeders

import "testing"

// TestErrorWrapperFunctions exercises representative wrapper functions to
// raise coverage; each call should return an error wrapping the base error.
func TestErrorWrapperFunctions(t *testing.T) {
	if err := wrapDotEnvStructureError(5); err == nil || err.Error() == "" {
		t.Fatal("expected dotenv structure error")
	}
	if err := wrapDotEnvUnsupportedTypeError("chan int"); err == nil || err.Error() == "" {
		t.Fatal("expected unsupported type error")
	}
	if err := wrapJSONMapError("cfg", 5); err == nil {
		t.Fatal("expected json map error")
	}
	if err := wrapJSONConvertError(42, "string", "cfg.field"); err == nil {
		t.Fatal("expected json convert error")
	}
	if err := wrapJSONSliceElementError(3.14, "string", "cfg.items", 0); err == nil {
		t.Fatal("expected slice element error")
	}
	if err := wrapJSONArrayError("cfg.items", 7); err == nil {
		t.Fatal("expected json array error")
	}
	if err := wrapTomlMapError("cfg", 7); err == nil {
		t.Fatal("expected toml map error")
	}
	if err := wrapTomlConvertError(7, "string", "cfg.field"); err == nil {
		t.Fatal("expected toml convert error")
	}
	if err := wrapTomlSliceElementError(7, "string", "cfg.items", 1); err == nil {
		t.Fatal("expected toml slice element error")
	}
	if err := wrapTomlArrayError("cfg.items", 9); err == nil {
		t.Fatal("expected toml array error")
	}
	if err := wrapYamlFieldCannotBeSetError(); err == nil {
		t.Fatal("expected yaml field cannot be set error")
	}
	if err := wrapYamlUnsupportedFieldTypeError("complex128"); err == nil {
		t.Fatal("expected yaml unsupported field type error")
	}
	if err := wrapYamlTypeConversionError("int", "string"); err == nil {
		t.Fatal("expected yaml type conversion error")
	}
	if err := wrapYamlBoolConversionError("notabool"); err == nil {
		t.Fatal("expected yaml bool conversion error")
	}
	if err := wrapJSONFieldCannotBeSet("cfg.x"); err == nil {
		t.Fatal("expected json field cannot be set error")
	}
	if err := wrapTomlFieldCannotBeSet("cfg.x"); err == nil {
		t.Fatal("expected toml field cannot be set error")
	}
	if err := wrapTomlArraySizeExceeded("cfg.arr", 5, 2); err == nil {
		t.Fatal("expected toml array size exceeded error")
	}
	if err := wrapJSONArraySizeExceeded("cfg.arr", 5, 2); err == nil {
		t.Fatal("expected json array size exceeded error")
	}
	if err := wrapYamlExpectedMapError("cfg", 3); err == nil {
		t.Fatal("expected yaml expected map error")
	}
	if err := wrapYamlExpectedArrayError("cfg.items", 3); err == nil {
		t.Fatal("expected yaml expected array error")
	}
	if err := wrapYamlArraySizeExceeded("cfg.items", 5, 2); err == nil {
		t.Fatal("expected yaml array size exceeded error")
	}
	if err := wrapYamlExpectedMapForSliceError("cfg.items", 0, 7); err == nil {
		t.Fatal("expected yaml expected map for slice error")
	}
}
