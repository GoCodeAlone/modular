package modular

import (
	"fmt"
	"reflect"
	"strings"
)

// FieldTracker interface allows feeders to report which fields they populate
type FieldTracker interface {
	// RecordFieldPopulation records that a field was populated by a feeder
	RecordFieldPopulation(fp FieldPopulation)

	// SetLogger sets the logger for the tracker
	SetLogger(logger Logger)
}

// FieldPopulation represents a single field population event
type FieldPopulation struct {
	FieldPath   string      // Full path to the field (e.g., "Connections.primary.DSN")
	FieldName   string      // Name of the field
	FieldType   string      // Type of the field
	FeederType  string      // Type of feeder that populated it
	SourceType  string      // Type of source (env, yaml, etc.)
	SourceKey   string      // Source key that was used (e.g., "DB_PRIMARY_DSN")
	Value       interface{} // Value that was set
	InstanceKey string      // Instance key for instance-aware fields
	SearchKeys  []string    // All keys that were searched for this field
	FoundKey    string      // The key that was actually found
}

// FieldTrackingFeeder interface allows feeders to support field tracking
type FieldTrackingFeeder interface {
	// SetFieldTracker sets the field tracker for this feeder
	SetFieldTracker(tracker FieldTracker)
}

// DefaultFieldTracker is a basic implementation of FieldTracker
type DefaultFieldTracker struct {
	FieldPopulations []FieldPopulation
	logger           Logger
}

// NewDefaultFieldTracker creates a new default field tracker
func NewDefaultFieldTracker() *DefaultFieldTracker {
	return &DefaultFieldTracker{
		FieldPopulations: make([]FieldPopulation, 0),
	}
}

// RecordFieldPopulation records a field population event
func (t *DefaultFieldTracker) RecordFieldPopulation(fp FieldPopulation) {
	t.FieldPopulations = append(t.FieldPopulations, fp)
	if t.logger != nil {
		t.logger.Debug("Field populated",
			"fieldPath", fp.FieldPath,
			"fieldName", fp.FieldName,
			"fieldType", fp.FieldType,
			"feederType", fp.FeederType,
			"sourceType", fp.SourceType,
			"sourceKey", fp.SourceKey,
			"value", fp.Value,
			"instanceKey", fp.InstanceKey,
			"searchKeys", strings.Join(fp.SearchKeys, ", "),
			"foundKey", fp.FoundKey,
		)
	}
}

// SetLogger sets the logger for the tracker
func (t *DefaultFieldTracker) SetLogger(logger Logger) {
	t.logger = logger
}

// GetFieldPopulation returns the population info for a specific field path
func (t *DefaultFieldTracker) GetFieldPopulation(fieldPath string) *FieldPopulation {
	for _, fp := range t.FieldPopulations {
		if fp.FieldPath == fieldPath {
			return &fp
		}
	}
	return nil
}

// GetPopulationsByFeeder returns all field populations by a specific feeder type
func (t *DefaultFieldTracker) GetPopulationsByFeeder(feederType string) []FieldPopulation {
	var result []FieldPopulation
	for _, fp := range t.FieldPopulations {
		if fp.FeederType == feederType {
			result = append(result, fp)
		}
	}
	return result
}

// GetPopulationsBySource returns all field populations by a specific source type
func (t *DefaultFieldTracker) GetPopulationsBySource(sourceType string) []FieldPopulation {
	var result []FieldPopulation
	for _, fp := range t.FieldPopulations {
		if fp.SourceType == sourceType {
			result = append(result, fp)
		}
	}
	return result
}

// StructStateDiffer captures before/after states to determine field changes
type StructStateDiffer struct {
	beforeState map[string]interface{}
	afterState  map[string]interface{}
	tracker     FieldTracker
	logger      Logger
}

// NewStructStateDiffer creates a new struct state differ
func NewStructStateDiffer(tracker FieldTracker, logger Logger) *StructStateDiffer {
	return &StructStateDiffer{
		beforeState: make(map[string]interface{}),
		afterState:  make(map[string]interface{}),
		tracker:     tracker,
		logger:      logger,
	}
}

// CaptureBeforeState captures the state before feeder processing
func (d *StructStateDiffer) CaptureBeforeState(structure interface{}, prefix string) {
	d.captureState(structure, prefix, d.beforeState)
	if d.logger != nil {
		d.logger.Debug("Captured before state", "prefix", prefix, "fieldCount", len(d.beforeState))
	}
}

// CaptureAfterStateAndDiff captures the state after feeder processing and computes diffs
func (d *StructStateDiffer) CaptureAfterStateAndDiff(structure interface{}, prefix string, feederType, sourceType string) {
	d.captureState(structure, prefix, d.afterState)
	if d.logger != nil {
		d.logger.Debug("Captured after state", "prefix", prefix, "fieldCount", len(d.afterState))
	}

	// Compute and record differences
	d.computeAndRecordDiffs(feederType, sourceType, prefix)
}

// captureState recursively captures all field values in a structure
func (d *StructStateDiffer) captureState(structure interface{}, prefix string, state map[string]interface{}) {
	rv := reflect.ValueOf(structure)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return
	}

	d.captureStructFields(rv, prefix, state)
}

// captureStructFields recursively captures all field values in a struct
func (d *StructStateDiffer) captureStructFields(rv reflect.Value, prefix string, state map[string]interface{}) {
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)

		if !field.CanInterface() {
			continue // Skip unexported fields
		}

		fieldPath := fieldType.Name
		if prefix != "" {
			fieldPath = prefix + "." + fieldType.Name
		}

		switch field.Kind() {
		case reflect.Struct:
			d.captureStructFields(field, fieldPath, state)
		case reflect.Ptr:
			if !field.IsNil() && field.Elem().Kind() == reflect.Struct {
				d.captureStructFields(field.Elem(), fieldPath, state)
			} else if !field.IsNil() {
				state[fieldPath] = field.Elem().Interface()
			}
		case reflect.Map:
			if !field.IsNil() {
				for _, key := range field.MapKeys() {
					mapValue := field.MapIndex(key)
					mapFieldPath := fieldPath + "." + key.String()
					if mapValue.Kind() == reflect.Struct {
						d.captureStructFields(mapValue, mapFieldPath, state)
					} else if mapValue.Kind() == reflect.Ptr && !mapValue.IsNil() && mapValue.Elem().Kind() == reflect.Struct {
						d.captureStructFields(mapValue.Elem(), mapFieldPath, state)
					} else {
						state[mapFieldPath] = mapValue.Interface()
					}
				}
			}
		case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
			reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
			reflect.Chan, reflect.Func, reflect.Interface, reflect.Slice, reflect.String, reflect.UnsafePointer:
			state[fieldPath] = field.Interface()
		}
	}
}

// computeAndRecordDiffs computes differences and records field populations
func (d *StructStateDiffer) computeAndRecordDiffs(feederType, sourceType, instanceKey string) {
	for fieldPath, afterValue := range d.afterState {
		beforeValue, existed := d.beforeState[fieldPath]

		// Check if field changed (either new or value changed)
		if !existed || !reflect.DeepEqual(beforeValue, afterValue) {
			// Parse field path to get field name and type
			parts := strings.Split(fieldPath, ".")
			fieldName := parts[len(parts)-1]

			// Determine field type
			fieldTypeStr := fmt.Sprintf("%T", afterValue)

			// Create field population record
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   fieldTypeStr,
				FeederType:  feederType,
				SourceType:  sourceType,
				SourceKey:   "detected_by_diff", // Will be enhanced by feeders that support direct tracking
				Value:       afterValue,
				InstanceKey: instanceKey,
				SearchKeys:  []string{}, // Will be enhanced by feeders that support direct tracking
				FoundKey:    "detected_by_diff",
			}

			if d.tracker != nil {
				d.tracker.RecordFieldPopulation(fp)
			}

			if d.logger != nil {
				d.logger.Debug("Detected field change via diff",
					"fieldPath", fieldPath,
					"fieldName", fieldName,
					"fieldType", fieldTypeStr,
					"feederType", feederType,
					"sourceType", sourceType,
					"value", afterValue,
					"instanceKey", instanceKey,
					"existed", existed,
					"beforeValue", beforeValue,
				)
			}
		}
	}
}

// Reset clears the captured states for reuse
func (d *StructStateDiffer) Reset() {
	d.beforeState = make(map[string]interface{})
	d.afterState = make(map[string]interface{})
}
