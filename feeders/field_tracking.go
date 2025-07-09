package feeders

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

// FieldTracker interface allows feeders to report which fields they populate
type FieldTracker interface {
	// RecordFieldPopulation records that a field was populated by a feeder
	RecordFieldPopulation(fp FieldPopulation)
}

// DefaultFieldTracker is a basic implementation of FieldTracker
type DefaultFieldTracker struct {
	populations []FieldPopulation
}

// NewDefaultFieldTracker creates a new DefaultFieldTracker
func NewDefaultFieldTracker() *DefaultFieldTracker {
	return &DefaultFieldTracker{
		populations: make([]FieldPopulation, 0),
	}
}

// RecordFieldPopulation records that a field was populated by a feeder
func (t *DefaultFieldTracker) RecordFieldPopulation(fp FieldPopulation) {
	t.populations = append(t.populations, fp)
}

// GetFieldPopulations returns all recorded field populations
func (t *DefaultFieldTracker) GetFieldPopulations() []FieldPopulation {
	return t.populations
}
