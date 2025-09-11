package modular

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultFieldTrackerAndQueries exercises the basic tracking & query helpers
func TestDefaultFieldTrackerAndQueries(t *testing.T) {
	tracker := NewDefaultFieldTracker()

	// Record two populations for same field; second should be considered most relevant
	tracker.RecordFieldPopulation(FieldPopulation{FieldPath: "Name", FieldName: "Name", FieldType: "string", FeederType: "envFeederA", SourceType: "env", SourceKey: "APP_NAME", Value: nil, FoundKey: ""})
	tracker.RecordFieldPopulation(FieldPopulation{FieldPath: "Name", FieldName: "Name", FieldType: "string", FeederType: "envFeederB", SourceType: "env", SourceKey: "APP_NAME", Value: "final", FoundKey: "APP_NAME"})
	tracker.RecordFieldPopulation(FieldPopulation{FieldPath: "Count", FieldName: "Count", FieldType: "int", FeederType: "yamlFeeder", SourceType: "yaml", SourceKey: "count", Value: 2, FoundKey: "count"})

	// GetFieldPopulation returns first occurrence
	first := tracker.GetFieldPopulation("Name")
	if assert.NotNil(t, first) {
		assert.Equal(t, "envFeederA", first.FeederType)
	}

	// Most relevant should return the second (with a concrete value & FoundKey)
	most := tracker.GetMostRelevantFieldPopulation("Name")
	if assert.NotNil(t, most) {
		assert.Equal(t, "envFeederB", most.FeederType)
		assert.Equal(t, "final", most.Value)
	}

	// Feeder filtering
	envFeederB := tracker.GetPopulationsByFeeder("envFeederB")
	assert.Len(t, envFeederB, 1)
	yamlFeeder := tracker.GetPopulationsByFeeder("yamlFeeder")
	assert.Len(t, yamlFeeder, 1)

	// Source filtering
	envSource := tracker.GetPopulationsBySource("env")
	assert.GreaterOrEqual(t, len(envSource), 2)
	yamlSource := tracker.GetPopulationsBySource("yaml")
	assert.Len(t, yamlSource, 1)
}

// TestStructStateDifferFullCoverage covers before/after capture, nested structures, maps, pointers and Reset.
func TestStructStateDifferFullCoverage(t *testing.T) {
	type Inner struct{ Value int }
	type Complex struct {
		Name   string
		Count  int
		Inner  Inner
		Ptr    *Inner
		Items  map[string]int
		Nested map[string]Inner
		Mixed  map[string]*Inner
		// unexported field should be ignored
		hidden string
	}

	initial := &Complex{
		Name:  "orig",
		Count: 1,
		Inner: Inner{Value: 10},
		Ptr:   nil,
		Items: map[string]int{"a": 1},
		Nested: map[string]Inner{
			"n1": {Value: 5},
		},
		Mixed: map[string]*Inner{
			"m1": {Value: 7},
		},
		hidden: "secret",
	}

	tracker := NewDefaultFieldTracker()
	differ := NewStructStateDiffer(tracker, nil)

	// Capture before state
	differ.CaptureBeforeState(initial, "")

	// Mutate several fields (including pointer creation & map additions)
	initial.Name = "changed"
	initial.Count = 2
	initial.Inner.Value = 11
	initial.Ptr = &Inner{Value: 42}
	initial.Items["b"] = 2                 // new key
	initial.Nested["n1"] = Inner{Value: 6} // changed nested struct
	initial.Mixed["m1"].Value = 8          // changed value inside pointer
	initial.Mixed["m2"] = &Inner{Value: 9} // new pointer entry

	differ.CaptureAfterStateAndDiff(initial, "", "customFeeder", "yaml")

	// Collect field paths recorded
	recorded := map[string]struct{}{}
	for _, fp := range tracker.FieldPopulations {
		recorded[fp.FieldPath] = struct{}{}
	}

	// Expected changed paths (order not guaranteed)
	expected := []string{
		"Name",
		"Count",
		"Inner.Value",
		"Ptr.Value",
		"Items.b",
		"Nested.n1.Value",
		"Mixed.m1.Value",
		"Mixed.m2.Value",
	}
	for _, path := range expected {
		if _, ok := recorded[path]; !ok {
			t.Fatalf("expected changed field %s to be recorded; got %#v", path, recorded)
		}
	}

	// Ensure feeder & source tagging applied
	for _, fp := range tracker.FieldPopulations {
		assert.Equal(t, "customFeeder", fp.FeederType)
		assert.Equal(t, "yaml", fp.SourceType)
		assert.Equal(t, "detected_by_diff", fp.SourceKey)
	}

	// Exercise captureState with non-struct (early return path)
	beforeCount := len(differ.beforeState)
	differ.captureState(42, "Number", differ.beforeState)
	assert.Equal(t, beforeCount, len(differ.beforeState), "non-struct capture should not add entries")

	// Test Reset reinitializes internal maps by performing another diff cycle
	differ.Reset()
	// Change again to produce new populations
	initial.Count = 3
	initial.Inner.Value = 12
	differ.CaptureBeforeState(initial, "")
	// Mutate both fields after capturing before state so both diffs are detected
	initial.Count = 4
	initial.Inner.Value = 13
	differ.CaptureAfterStateAndDiff(initial, "", "customFeeder", "yaml")

	// We should now have additional populations beyond original set
	foundCountChange := 0
	for _, fp := range tracker.FieldPopulations {
		if fp.FieldPath == "Count" && fp.Value == 4 { // from second cycle
			foundCountChange++
		}
		if fp.FieldPath == "Inner.Value" && fp.Value == 13 { // second cycle change
			foundCountChange++
		}
	}
	require.Equal(t, 2, foundCountChange, "expected second cycle diff populations recorded")
}
