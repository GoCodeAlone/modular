package modular

import (
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsZeroValueCoversKinds ensures isZeroValue logic over key kinds.
func TestIsZeroValueCoversKinds(t *testing.T) {
	cases := []struct {
		val  interface{}
		kind reflect.Kind
		zero bool
	}{
		{"", reflect.String, true},
		{"x", reflect.String, false},
		{0, reflect.Int, true},
		{1, reflect.Int, false},
		{uint(0), reflect.Uint, true},
		{uint(2), reflect.Uint, false},
		{0.0, reflect.Float64, true},
		{1.1, reflect.Float64, false},
		{false, reflect.Bool, true},
		{true, reflect.Bool, false},
		{[]string{}, reflect.Slice, true},
		{[]string{"a"}, reflect.Slice, false},
		{map[string]int{}, reflect.Map, true},
		{map[string]int{"k": 1}, reflect.Map, false},
		{complex64(0), reflect.Complex64, true},
		{complex64(1 + 2i), reflect.Complex64, false},
	}
	for _, c := range cases {
		v := reflect.ValueOf(c.val)
		got := isZeroValue(v)
		assert.Equalf(t, c.zero, got, "unexpected zero detection for kind %s value %#v", c.kind, c.val)
	}
}

// TestHandleUnsupportedDefaultTypeErrors covers each branch for unsupported kinds.
func TestHandleUnsupportedDefaultTypeErrors(t *testing.T) {
	kinds := []reflect.Kind{reflect.Invalid, reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Ptr, reflect.Struct, reflect.UnsafePointer, reflect.Complex64}
	for _, k := range kinds {
		err := handleUnsupportedDefaultType(k)
		assert.Error(t, err, "expected error for kind %s", k)
		assert.ErrorIs(t, err, ErrUnsupportedTypeForDefault)
	}
	// A primitive supported by setDefaultValue path should also produce an error when routed directly
	err := handleUnsupportedDefaultType(reflect.String)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrUnsupportedTypeForDefault)
}

// helper to get addressable value of specific kind
func addrValue(t *testing.T, kind reflect.Kind) reflect.Value {
	switch kind {
	case reflect.Int:
		v := int64(0)
		return reflect.ValueOf(&v).Elem()
	case reflect.Uint:
		v := uint64(0)
		return reflect.ValueOf(&v).Elem()
	case reflect.Float64:
		v := float64(0)
		return reflect.ValueOf(&v).Elem()
	default:
		t.Fatalf("unsupported helper kind %s", kind)
	}
	return reflect.Value{}
}

// TestSetDefaultIntUintFloatSuccessAndOverflow covers success and overflow/error paths.
func TestSetDefaultIntUintFloatSuccessAndOverflow(t *testing.T) {
	// Int success
	intVal := addrValue(t, reflect.Int)
	assert.NoError(t, setDefaultInt(intVal, 42))
	assert.Equal(t, int64(42), intVal.Int())
	// Int overflow (simulate by using max int8 target)
	var small int8
	smallRV := reflect.ValueOf(&small).Elem()
	assert.NoError(t, setDefaultInt(smallRV, 127))
	err := setDefaultInt(smallRV, 128)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDefaultValueOverflowsInt)

	// Uint success
	uintVal := addrValue(t, reflect.Uint)
	assert.NoError(t, setDefaultUint(uintVal, 99))
	assert.Equal(t, uint64(99), uintVal.Uint())
	// Uint overflow (simulate by using uint8)
	var usmall uint8
	usmallRV := reflect.ValueOf(&usmall).Elem()
	assert.NoError(t, setDefaultUint(usmallRV, 255))
	err = setDefaultUint(usmallRV, 256)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDefaultValueOverflowsUint)

	// Float success
	floatVal := addrValue(t, reflect.Float64)
	assert.NoError(t, setDefaultFloat(floatVal, 3.14))
	assert.InDelta(t, 3.14, floatVal.Float(), 0.0001)

	// Float overflow - create float32 target and set a value exceeding its range
	var f32 float32
	f32RV := reflect.ValueOf(&f32).Elem()
	// Use a value larger than max float32 (~3.4e38)
	err = setDefaultFloat(f32RV, math.MaxFloat64)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDefaultValueOverflowsFloat)
}
