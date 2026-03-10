package modular

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_createTempConfig(t *testing.T) {
	t.Parallel()
	t.Run("with pointer", func(t *testing.T) {
		originalCfg := &testCfg{Str: "test", Num: 42}
		tempCfg, info, err := createTempConfig(originalCfg)

		require.NoError(t, err)
		require.NotNil(t, tempCfg)
		assert.True(t, info.isPtr)
		assert.Equal(t, reflect.ValueOf(originalCfg).Type(), info.tempVal.Type())
	})

	t.Run("with non-pointer", func(t *testing.T) {
		originalCfg := testCfg{Str: "test", Num: 42}
		tempCfg, info, err := createTempConfig(originalCfg)

		require.NoError(t, err)
		require.NotNil(t, tempCfg)
		assert.False(t, info.isPtr)
		assert.Equal(t, reflect.PointerTo(reflect.ValueOf(originalCfg).Type()), info.tempVal.Type())
	})

	t.Run("maps and slices are shallow copied (default behavior)", func(t *testing.T) {
		type ConfigWithMaps struct {
			Name     string
			Settings map[string]string
			Tags     []string
		}

		// Create an original config with initialized maps and slices
		originalCfg := &ConfigWithMaps{
			Name: "original",
			Settings: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			Tags: []string{"tag1", "tag2"},
		}

		// Create temp config
		tempCfg, info, err := createTempConfig(originalCfg)
		require.NoError(t, err)
		require.NotNil(t, tempCfg)

		tempCfgTyped := tempCfg.(*ConfigWithMaps)

		// Verify initial values are copied
		assert.Equal(t, "original", tempCfgTyped.Name)
		assert.Equal(t, "value1", tempCfgTyped.Settings["key1"])
		assert.Equal(t, "tag1", tempCfgTyped.Tags[0])

		// Modify the temp config's maps and slices
		tempCfgTyped.Settings["key1"] = "MODIFIED"
		tempCfgTyped.Settings["newkey"] = "newvalue"
		tempCfgTyped.Tags[0] = "MODIFIED"

		// IMPORTANT: createTempConfig does a SHALLOW copy using reflect.Value.Set()
		// This means maps and slices are shared between original and temp
		assert.Equal(t, "MODIFIED", originalCfg.Settings["key1"],
			"Original config's map IS affected by temp config modifications (shallow copy)")
		assert.Contains(t, originalCfg.Settings, "newkey",
			"Original config's map IS affected by temp config modifications (shallow copy)")
		assert.Equal(t, "MODIFIED", originalCfg.Tags[0],
			"Original config's slice IS affected by temp config modifications (shallow copy)")

		// Verify the info struct is correct
		assert.True(t, info.isPtr)
	})

	t.Run("deep copy isolates maps and slices (createTempConfigDeep)", func(t *testing.T) {
		type ConfigWithMaps struct {
			Name     string
			Settings map[string]string
			Tags     []string
		}

		// Create an original config with initialized maps and slices
		originalCfg := &ConfigWithMaps{
			Name: "original",
			Settings: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			Tags: []string{"tag1", "tag2"},
		}

		// Create temp config using DEEP copy
		tempCfg, info, err := createTempConfigDeep(originalCfg)
		require.NoError(t, err)
		require.NotNil(t, tempCfg)

		tempCfgTyped := tempCfg.(*ConfigWithMaps)

		// Verify initial values are copied
		assert.Equal(t, "original", tempCfgTyped.Name)
		assert.Equal(t, "value1", tempCfgTyped.Settings["key1"])
		assert.Equal(t, "tag1", tempCfgTyped.Tags[0])

		// Modify the temp config's maps and slices
		tempCfgTyped.Settings["key1"] = "MODIFIED"
		tempCfgTyped.Settings["newkey"] = "newvalue"
		tempCfgTyped.Tags[0] = "MODIFIED"

		// Deep copy ensures isolation - original should NOT be affected
		assert.Equal(t, "value1", originalCfg.Settings["key1"],
			"Original config's map should NOT be affected (deep copy)")
		assert.NotContains(t, originalCfg.Settings, "newkey",
			"Original config's map should NOT be affected (deep copy)")
		assert.Equal(t, "tag1", originalCfg.Tags[0],
			"Original config's slice should NOT be affected (deep copy)")

		// Verify the info struct is correct
		assert.True(t, info.isPtr)
	})
}

func Test_updateConfig(t *testing.T) {
	t.Parallel()
	t.Run("with pointer config", func(t *testing.T) {
		originalCfg := &testCfg{Str: "old", Num: 0}
		tempCfg := &testCfg{Str: "new", Num: 42}

		mockLogger := new(MockLogger)
		app := &StdApplication{logger: mockLogger}

		origInfo := configInfo{
			originalVal: reflect.ValueOf(originalCfg),
			tempVal:     reflect.ValueOf(tempCfg),
			isPtr:       true,
		}

		updateConfig(app, origInfo)

		// Check the original config was updated
		assert.Equal(t, "new", originalCfg.Str)
		assert.Equal(t, 42, originalCfg.Num)
	})

	t.Run("with non-pointer config", func(t *testing.T) {
		originalCfg := testCfg{Str: "old", Num: 0}
		tempCfgPtr, origInfo, err := createTempConfig(originalCfg)
		require.NoError(t, err)
		tempCfgPtr.(*testCfg).Str = "new"
		tempCfgPtr.(*testCfg).Num = 42

		mockLogger := new(MockLogger)
		mockLogger.On("Debug",
			"Creating new provider with updated config (original was non-pointer)",
			[]any(nil)).Return()
		app := &StdApplication{
			logger:      mockLogger,
			cfgProvider: NewStdConfigProvider(originalCfg),
		}

		updateConfig(app, origInfo)

		// Check the updated provider from the app (not the original provider reference)
		updated := app.cfgProvider.GetConfig()
		assert.Equal(t, reflect.Struct, reflect.ValueOf(updated).Kind())
		assert.Equal(t, "new", updated.(testCfg).Str)
		assert.Equal(t, 42, updated.(testCfg).Num)
		mockLogger.AssertExpectations(t)
	})
}

func Test_updateSectionConfig(t *testing.T) {
	t.Parallel()
	t.Run("with pointer section config", func(t *testing.T) {
		originalCfg := &testSectionCfg{Enabled: false, Name: "old"}
		tempCfg := &testSectionCfg{Enabled: true, Name: "new"}

		mockLogger := new(MockLogger)
		app := &StdApplication{
			logger:      mockLogger,
			cfgSections: make(map[string]ConfigProvider),
		}
		app.cfgSections["test"] = NewStdConfigProvider(originalCfg)

		sectionInfo := configInfo{
			originalVal: reflect.ValueOf(originalCfg),
			tempVal:     reflect.ValueOf(tempCfg),
			isPtr:       true,
		}

		updateSectionConfig(app, "test", sectionInfo)

		// Check the original config was updated
		assert.True(t, originalCfg.Enabled)
		assert.Equal(t, "new", originalCfg.Name)
	})

	t.Run("with non-pointer section config", func(t *testing.T) {
		originalCfg := testSectionCfg{Enabled: false, Name: "old"}
		tempCfgPtr, sectionInfo, err := createTempConfig(originalCfg)
		require.NoError(t, err)

		// Cast and update the temp config
		tempCfgPtr.(*testSectionCfg).Enabled = true
		tempCfgPtr.(*testSectionCfg).Name = "new"

		mockLogger := new(MockLogger)
		mockLogger.On("Debug", "Creating new provider for section", []any{"section", "test"}).Return()

		app := &StdApplication{
			logger:      mockLogger,
			cfgSections: make(map[string]ConfigProvider),
		}
		app.cfgSections["test"] = NewStdConfigProvider(originalCfg)

		updateSectionConfig(app, "test", sectionInfo)

		// Check a new provider was created
		sectCfg := app.cfgSections["test"].GetConfig()
		assert.True(t, sectCfg.(testSectionCfg).Enabled)
		assert.Equal(t, "new", sectCfg.(testSectionCfg).Name)
		mockLogger.AssertExpectations(t)
	})
}

// TestDeepCopyValue_Maps tests deep copying of maps
func TestDeepCopyValue_Maps(t *testing.T) {
	t.Parallel()

	t.Run("simple map of strings", func(t *testing.T) {
		src := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstMap := dst.Interface().(map[string]string)
		assert.Equal(t, "value1", dstMap["key1"])
		assert.Equal(t, "value2", dstMap["key2"])

		// Verify it's a deep copy by modifying the source
		src["key1"] = "modified"
		assert.Equal(t, "value1", dstMap["key1"], "Destination should not be affected by source modification")
	})

	t.Run("map with string slice values", func(t *testing.T) {
		src := map[string][]string{
			"list1": {"a", "b", "c"},
			"list2": {"x", "y", "z"},
		}

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstMap := dst.Interface().(map[string][]string)
		assert.Equal(t, []string{"a", "b", "c"}, dstMap["list1"])
		assert.Equal(t, []string{"x", "y", "z"}, dstMap["list2"])

		// Verify it's a deep copy by modifying the source slice
		src["list1"][0] = "modified"
		assert.Equal(t, "a", dstMap["list1"][0], "Destination slice should not be affected")
	})

	t.Run("nested maps", func(t *testing.T) {
		src := map[string]map[string]int{
			"group1": {"a": 1, "b": 2},
			"group2": {"x": 10, "y": 20},
		}

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstMap := dst.Interface().(map[string]map[string]int)
		assert.Equal(t, 1, dstMap["group1"]["a"])
		assert.Equal(t, 20, dstMap["group2"]["y"])

		// Verify it's a deep copy
		src["group1"]["a"] = 999
		assert.Equal(t, 1, dstMap["group1"]["a"], "Nested map should not be affected")
	})

	t.Run("nil map", func(t *testing.T) {
		var src map[string]string = nil

		dst := reflect.New(reflect.TypeFor[map[string]string]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		// For nil map, deepCopyValue returns early without modifying dst
		// dst remains as zero value which is nil for maps
		assert.True(t, !dst.IsValid() || dst.IsNil(), "Destination should remain nil for nil source")
	})
}

// TestDeepCopyValue_Slices tests deep copying of slices
func TestDeepCopyValue_Slices(t *testing.T) {
	t.Parallel()

	t.Run("simple slice of integers", func(t *testing.T) {
		src := []int{1, 2, 3, 4, 5}

		dst := reflect.New(reflect.TypeFor[[]int]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstSlice := dst.Interface().([]int)
		assert.Equal(t, []int{1, 2, 3, 4, 5}, dstSlice)

		// Verify it's a deep copy
		src[0] = 999
		assert.Equal(t, 1, dstSlice[0], "Destination should not be affected")
	})

	t.Run("slice of strings", func(t *testing.T) {
		src := []string{"hello", "world"}

		dst := reflect.New(reflect.TypeFor[[]string]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstSlice := dst.Interface().([]string)
		assert.Equal(t, []string{"hello", "world"}, dstSlice)

		src[0] = "modified"
		assert.Equal(t, "hello", dstSlice[0])
	})

	t.Run("slice of maps", func(t *testing.T) {
		src := []map[string]int{
			{"a": 1, "b": 2},
			{"x": 10, "y": 20},
		}

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstSlice := dst.Interface().([]map[string]int)
		assert.Equal(t, 1, dstSlice[0]["a"])
		assert.Equal(t, 20, dstSlice[1]["y"])

		// Verify it's a deep copy
		src[0]["a"] = 999
		assert.Equal(t, 1, dstSlice[0]["a"], "Nested map in slice should not be affected")
	})

	t.Run("nil slice", func(t *testing.T) {
		var src []string = nil

		dst := reflect.New(reflect.TypeFor[[]string]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		// For nil slice, deepCopyValue returns early without modifying dst
		// dst remains as zero value which is nil for slices
		assert.True(t, !dst.IsValid() || dst.IsNil(), "Destination should remain nil for nil source")
	})
}

// TestDeepCopyValue_Pointers tests deep copying of pointers
func TestDeepCopyValue_Pointers(t *testing.T) {
	t.Parallel()

	t.Run("pointer to string", func(t *testing.T) {
		str := "original"
		src := &str

		dst := reflect.New(reflect.TypeFor[*string]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstPtr := dst.Interface().(*string)
		assert.Equal(t, "original", *dstPtr)

		// Verify it's a deep copy
		*src = "modified"
		assert.Equal(t, "original", *dstPtr, "Destination should not be affected")
	})

	t.Run("pointer to struct", func(t *testing.T) {
		type TestStruct struct {
			Name  string
			Value int
		}

		src := &TestStruct{Name: "test", Value: 42}

		dst := reflect.New(reflect.TypeFor[*TestStruct]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstPtr := dst.Interface().(*TestStruct)
		assert.Equal(t, "test", dstPtr.Name)
		assert.Equal(t, 42, dstPtr.Value)

		// Verify it's a deep copy
		src.Name = "modified"
		assert.Equal(t, "test", dstPtr.Name)
	})

	t.Run("nil pointer", func(t *testing.T) {
		var src *string = nil

		dst := reflect.New(reflect.TypeFor[*string]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		// For nil pointer, deepCopyValue returns early without modifying dst
		// dst remains as zero value which is nil for pointers
		assert.True(t, !dst.IsValid() || dst.IsNil(), "Destination should remain nil for nil source")
	})
}

// TestDeepCopyValue_Structs tests deep copying of structs
func TestDeepCopyValue_Structs(t *testing.T) {
	t.Parallel()

	t.Run("simple struct", func(t *testing.T) {
		type SimpleStruct struct {
			Name string
			Age  int
		}

		src := SimpleStruct{Name: "John", Age: 30}

		dst := reflect.New(reflect.TypeFor[SimpleStruct]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstStruct := dst.Interface().(SimpleStruct)
		assert.Equal(t, "John", dstStruct.Name)
		assert.Equal(t, 30, dstStruct.Age)
	})

	t.Run("struct with map field", func(t *testing.T) {
		type ConfigStruct struct {
			Name     string
			Settings map[string]string
		}

		src := ConfigStruct{
			Name:     "config1",
			Settings: map[string]string{"key1": "value1", "key2": "value2"},
		}

		dst := reflect.New(reflect.TypeFor[ConfigStruct]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstStruct := dst.Interface().(ConfigStruct)
		assert.Equal(t, "config1", dstStruct.Name)
		assert.Equal(t, "value1", dstStruct.Settings["key1"])

		// Verify it's a deep copy - THIS TESTS THE KEY BUG FIX
		src.Settings["key1"] = "modified"
		assert.Equal(t, "value1", dstStruct.Settings["key1"], "Map in struct should not be affected")
	})

	t.Run("struct with slice field", func(t *testing.T) {
		type ListStruct struct {
			Name  string
			Items []string
		}

		src := ListStruct{
			Name:  "list1",
			Items: []string{"a", "b", "c"},
		}

		dst := reflect.New(reflect.TypeFor[ListStruct]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstStruct := dst.Interface().(ListStruct)
		assert.Equal(t, "list1", dstStruct.Name)
		assert.Equal(t, []string{"a", "b", "c"}, dstStruct.Items)

		// Verify it's a deep copy
		src.Items[0] = "modified"
		assert.Equal(t, "a", dstStruct.Items[0], "Slice in struct should not be affected")
	})

	t.Run("nested struct", func(t *testing.T) {
		type InnerStruct struct {
			Value int
		}
		type OuterStruct struct {
			Name  string
			Inner InnerStruct
		}

		src := OuterStruct{
			Name:  "outer",
			Inner: InnerStruct{Value: 42},
		}

		dst := reflect.New(reflect.TypeFor[OuterStruct]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstStruct := dst.Interface().(OuterStruct)
		assert.Equal(t, "outer", dstStruct.Name)
		assert.Equal(t, 42, dstStruct.Inner.Value)
	})

	t.Run("struct with unexported fields", func(t *testing.T) {
		type StructWithPrivate struct {
			Public  string
			private int
		}

		src := StructWithPrivate{Public: "visible", private: 42}

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		// Should not panic even with unexported fields
		require.NotPanics(t, func() {
			deepCopyValue(dst, reflect.ValueOf(src))
		})

		dstStruct := dst.Interface().(StructWithPrivate)
		assert.Equal(t, "visible", dstStruct.Public)
	})
}

// TestDeepCopyValue_BasicTypes tests deep copying of basic types
func TestDeepCopyValue_BasicTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
	}{
		{"int", 42},
		{"int64", int64(123456789)},
		{"float64", 3.14159},
		{"string", "hello world"},
		{"bool", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := reflect.ValueOf(tt.value)
			dst := reflect.New(src.Type()).Elem()

			deepCopyValue(dst, src)

			assert.Equal(t, tt.value, dst.Interface())
		})
	}
}

// TestDeepCopyValue_ComplexStructures tests deep copying of complex nested structures
func TestDeepCopyValue_ComplexStructures(t *testing.T) {
	t.Parallel()

	type ComplexConfig struct {
		Name            string
		BackendServices map[string]string
		Features        map[string]bool
		AllowedIPs      []string
	}

	src := ComplexConfig{
		Name: "tenant1",
		BackendServices: map[string]string{
			"api":    "https://api.example.com",
			"legacy": "https://legacy.example.com",
		},
		Features: map[string]bool{
			"feature1": true,
			"feature2": false,
		},
		AllowedIPs: []string{"192.168.1.1", "10.0.0.1"},
	}

	dst := reflect.New(reflect.TypeFor[ComplexConfig]()).Elem()
	deepCopyValue(dst, reflect.ValueOf(src))

	dstConfig := dst.Interface().(ComplexConfig)

	// Verify all fields copied correctly
	assert.Equal(t, "tenant1", dstConfig.Name)
	assert.Equal(t, "https://api.example.com", dstConfig.BackendServices["api"])
	assert.Equal(t, "https://legacy.example.com", dstConfig.BackendServices["legacy"])
	assert.True(t, dstConfig.Features["feature1"])
	assert.False(t, dstConfig.Features["feature2"])
	assert.Equal(t, []string{"192.168.1.1", "10.0.0.1"}, dstConfig.AllowedIPs)

	// Verify deep copy by modifying source
	src.BackendServices["api"] = "https://modified.example.com"
	src.Features["feature1"] = false
	src.AllowedIPs[0] = "1.1.1.1"

	// Destination should NOT be affected (isolation is preserved)
	assert.Equal(t, "https://api.example.com", dstConfig.BackendServices["api"], "BackendServices map should be deep copied")
	assert.True(t, dstConfig.Features["feature1"], "Features map should be deep copied")
	assert.Equal(t, "192.168.1.1", dstConfig.AllowedIPs[0], "AllowedIPs slice should be deep copied")
}

// TestDeepCopyValue_Arrays tests deep copying of fixed-size arrays
func TestDeepCopyValue_Arrays(t *testing.T) {
	t.Parallel()

	t.Run("array of integers", func(t *testing.T) {
		src := [5]int{1, 2, 3, 4, 5}

		dst := reflect.New(reflect.TypeFor[[5]int]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstArray := dst.Interface().([5]int)
		assert.Equal(t, [5]int{1, 2, 3, 4, 5}, dstArray)

		// Arrays are value types in Go, but let's verify the copy works
		src[0] = 999
		assert.Equal(t, 1, dstArray[0], "Destination array should not be affected")
	})

	t.Run("array of strings", func(t *testing.T) {
		src := [3]string{"foo", "bar", "baz"}

		dst := reflect.New(reflect.TypeFor[[3]string]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstArray := dst.Interface().([3]string)
		assert.Equal(t, [3]string{"foo", "bar", "baz"}, dstArray)

		src[1] = "modified"
		assert.Equal(t, "bar", dstArray[1])
	})

	t.Run("array of pointers", func(t *testing.T) {
		str1, str2 := "value1", "value2"
		src := [2]*string{&str1, &str2}

		dst := reflect.New(reflect.TypeFor[[2]*string]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstArray := dst.Interface().([2]*string)
		assert.Equal(t, "value1", *dstArray[0])
		assert.Equal(t, "value2", *dstArray[1])

		// Verify deep copy - modifying source pointer values shouldn't affect destination
		*src[0] = "modified"
		assert.Equal(t, "value1", *dstArray[0], "Array of pointers should be deep copied")
	})
}

// TestDeepCopyValue_Interfaces tests deep copying of interface values
func TestDeepCopyValue_Interfaces(t *testing.T) {
	t.Parallel()

	t.Run("interface with concrete string", func(t *testing.T) {
		var src any = "hello"

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstValue := dst.Interface()
		assert.Equal(t, "hello", dstValue)
	})

	t.Run("interface with concrete map", func(t *testing.T) {
		var src any = map[string]int{"a": 1, "b": 2}

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstValue := dst.Interface().(map[string]int)
		assert.Equal(t, 1, dstValue["a"])
		assert.Equal(t, 2, dstValue["b"])

		// Verify it's a deep copy
		srcMap := src.(map[string]int)
		srcMap["a"] = 999
		assert.Equal(t, 1, dstValue["a"], "Interface containing map should be deep copied")
	})

	t.Run("struct with interface field", func(t *testing.T) {
		type ConfigWithInterface struct {
			Name string
			Data any
		}

		src := ConfigWithInterface{
			Name: "test",
			Data: map[string]int{"count": 42},
		}

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstValue := dst.Interface().(ConfigWithInterface)
		assert.Equal(t, "test", dstValue.Name)
		assert.Equal(t, 42, dstValue.Data.(map[string]int)["count"])

		// Verify deep copy
		srcMap := src.Data.(map[string]int)
		srcMap["count"] = 999
		assert.Equal(t, 42, dstValue.Data.(map[string]int)["count"], "Interface field containing map should be deep copied")
	})

	t.Run("struct with nil interface field", func(t *testing.T) {
		type ConfigWithInterface struct {
			Name string
			Data any
		}

		src := ConfigWithInterface{
			Name: "test",
			Data: nil,
		}

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstValue := dst.Interface().(ConfigWithInterface)
		assert.Equal(t, "test", dstValue.Name)
		assert.Nil(t, dstValue.Data, "Nil interface field should remain nil")
	})

	t.Run("interface with struct", func(t *testing.T) {
		type TestStruct struct {
			Value int
			Data  map[string]string
		}

		var src any = TestStruct{
			Value: 42,
			Data:  map[string]string{"key": "value"},
		}

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstValue := dst.Interface().(TestStruct)
		assert.Equal(t, 42, dstValue.Value)
		assert.Equal(t, "value", dstValue.Data["key"])

		// Verify deep copy of the map inside the struct
		srcStruct := src.(TestStruct)
		srcStruct.Data["key"] = "modified"
		assert.Equal(t, "value", dstValue.Data["key"], "Interface with struct containing map should be deep copied")
	})
}

// TestDeepCopyValue_Channels tests copying of channels (by reference)
func TestDeepCopyValue_Channels(t *testing.T) {
	t.Parallel()

	t.Run("channel of integers", func(t *testing.T) {
		src := make(chan int, 2)
		src <- 42
		src <- 100

		dst := reflect.New(reflect.TypeFor[chan int]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstChan := dst.Interface().(chan int)

		// Channels are copied by reference, so they should be the same channel
		assert.Equal(t, 42, <-dstChan, "Channel should be copied by reference")
		assert.Equal(t, 100, <-dstChan, "Channel should be copied by reference")
	})

	t.Run("nil channel", func(t *testing.T) {
		var src chan string = nil

		dst := reflect.New(reflect.TypeFor[chan string]()).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstChan := dst.Interface().(chan string)
		assert.Nil(t, dstChan, "Nil channel should remain nil")
	})
}

// TestDeepCopyValue_Functions tests copying of functions (by reference)
func TestDeepCopyValue_Functions(t *testing.T) {
	t.Parallel()

	t.Run("function value", func(t *testing.T) {
		callCount := 0
		src := func(x int) int {
			callCount++
			return x * 2
		}

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstFunc := dst.Interface().(func(int) int)

		// Functions are copied by reference, so calling either increments the same counter
		assert.Equal(t, 10, dstFunc(5))
		assert.Equal(t, 1, callCount, "Function should be copied by reference")

		assert.Equal(t, 20, src(10))
		assert.Equal(t, 2, callCount, "Both function references share state")
	})

	t.Run("nil function", func(t *testing.T) {
		var src func(int) int = nil

		dst := reflect.New(reflect.TypeOf(src)).Elem()
		deepCopyValue(dst, reflect.ValueOf(src))

		dstFunc := dst.Interface().(func(int) int)
		assert.Nil(t, dstFunc, "Nil function should remain nil")
	})
}

// TestDeepCopyValue_Invalid tests handling of invalid reflect values
func TestDeepCopyValue_Invalid(t *testing.T) {
	t.Parallel()

	t.Run("invalid value", func(t *testing.T) {
		var src reflect.Value // Invalid (zero value)

		dst := reflect.New(reflect.TypeFor[string]()).Elem()

		// Should not panic
		require.NotPanics(t, func() {
			deepCopyValue(dst, src)
		})
	})
}

// TestTenantConfigIsolation tests that tenant configurations are properly isolated
// and do not share map references when copied (the bug that was fixed in commit dc66902)
func TestTenantConfigIsolation(t *testing.T) {
	t.Parallel()

	// This test simulates the actual bug scenario:
	// 1. A config struct is created with initialized maps
	// 2. That config is copied using createTempConfig (which happens during tenant loading)
	// 3. If shallow copy is used (the bug), modifying one copy affects the other

	// Create a base config with maps - this simulates a default/template config
	baseConfig := &TestTenantConfig{
		Name:        "BaseConfig",
		Environment: "default",
		Features: map[string]bool{
			"feature1": true,
			"feature2": false,
		},
	}

	// Simulate what happens when loading tenant configs:
	// Use createTempConfigDeep to make an ISOLATED copy (fixes the bug)
	// NOTE: The original bug used createTempConfig which did shallow copy
	tenantACfg, _, err := createTempConfigDeep(baseConfig)
	require.NoError(t, err)
	tenantATyped := tenantACfg.(*TestTenantConfig)

	tenantBCfg, _, err := createTempConfigDeep(baseConfig)
	require.NoError(t, err)
	tenantBTyped := tenantBCfg.(*TestTenantConfig)

	// At this point, both tenants have copies of the base config
	// Verify initial state
	assert.Equal(t, "BaseConfig", tenantATyped.Name)
	assert.Equal(t, "BaseConfig", tenantBTyped.Name)
	assert.True(t, tenantATyped.Features["feature1"])
	assert.True(t, tenantBTyped.Features["feature1"])

	// THE CRITICAL TEST: Modify tenant A's map
	tenantATyped.Features["feature1"] = false
	tenantATyped.Features["new_feature_A"] = true
	tenantATyped.Name = "TenantA"

	// Modify tenant B's map
	tenantBTyped.Features["feature2"] = true
	tenantBTyped.Features["new_feature_B"] = true
	tenantBTyped.Name = "TenantB"

	// If deep copy is working:
	// - TenantA and TenantB should have independent maps
	// - Changes to one should NOT affect the other
	// - The base config should also remain unchanged

	// If shallow copy (the bug):
	// - All three would share the same map reference
	// - Changes to any would affect all

	// Verify tenant A has its own changes
	assert.False(t, tenantATyped.Features["feature1"], "TenantA should have feature1=false")
	assert.True(t, tenantATyped.Features["new_feature_A"], "TenantA should have new_feature_A")
	assert.False(t, tenantATyped.Features["new_feature_B"], "TenantA should NOT have TenantB's features")
	assert.Equal(t, "TenantA", tenantATyped.Name)

	// Verify tenant B has its own changes
	assert.True(t, tenantBTyped.Features["feature1"], "TenantB should still have feature1=true (unaffected by TenantA)")
	assert.True(t, tenantBTyped.Features["feature2"], "TenantB should have feature2=true")
	assert.True(t, tenantBTyped.Features["new_feature_B"], "TenantB should have new_feature_B")
	assert.False(t, tenantBTyped.Features["new_feature_A"], "TenantB should NOT have TenantA's features")
	assert.Equal(t, "TenantB", tenantBTyped.Name)

	// Verify base config remains unchanged
	assert.Equal(t, "BaseConfig", baseConfig.Name, "Base config name should be unchanged")
	assert.True(t, baseConfig.Features["feature1"], "Base config should still have feature1=true")
	assert.False(t, baseConfig.Features["feature2"], "Base config should still have feature2=false")
	assert.False(t, baseConfig.Features["new_feature_A"], "Base config should NOT have TenantA's features")
	assert.False(t, baseConfig.Features["new_feature_B"], "Base config should NOT have TenantB's features")

	t.Log("SUCCESS: Tenant configurations are properly isolated via deep copy - no shared map references")
}

// TestTenantConfigIsolation_ShallowCopyBug demonstrates the bug that occurs with shallow copy
// This test intentionally uses createTempConfig (shallow copy) to show the problematic behavior
func TestTenantConfigIsolation_ShallowCopyBug(t *testing.T) {
	t.Parallel()

	// This test demonstrates the BUG: when using createTempConfig (shallow copy),
	// all configs share the same map reference

	baseConfig := &TestTenantConfig{
		Name:        "BaseConfig",
		Environment: "default",
		Features: map[string]bool{
			"feature1": true,
			"feature2": false,
		},
	}

	// Using createTempConfig (shallow copy) - this is the bug
	tenantACfg, _, err := createTempConfig(baseConfig)
	require.NoError(t, err)
	tenantATyped := tenantACfg.(*TestTenantConfig)

	tenantBCfg, _, err := createTempConfig(baseConfig)
	require.NoError(t, err)
	tenantBTyped := tenantBCfg.(*TestTenantConfig)

	// Modify tenant A's map
	tenantATyped.Features["feature1"] = false
	tenantATyped.Features["new_feature_A"] = true

	// THE BUG: Because of shallow copy, modifying tenantA's map also affects:
	// 1. tenantB's map (they share the same reference)
	// 2. baseConfig's map (they all share the same reference)

	// Demonstrate the bug: tenantB sees tenantA's changes (WRONG!)
	assert.False(t, tenantBTyped.Features["feature1"],
		"BUG: TenantB sees TenantA's changes because they share the map reference")
	assert.True(t, tenantBTyped.Features["new_feature_A"],
		"BUG: TenantB has TenantA's features because they share the map reference")

	// Demonstrate the bug: baseConfig sees tenantA's changes (WRONG!)
	assert.False(t, baseConfig.Features["feature1"],
		"BUG: Base config was modified by TenantA because they share the map reference")
	assert.True(t, baseConfig.Features["new_feature_A"],
		"BUG: Base config has TenantA's features because they share the map reference")

	t.Log("BUG DEMONSTRATED: Shallow copy causes tenant configs to share map references")
}
