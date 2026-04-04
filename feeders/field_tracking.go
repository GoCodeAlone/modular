package feeders

import "sync"

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
	mu          sync.Mutex
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
	t.mu.Lock()
	t.populations = append(t.populations, fp)
	t.mu.Unlock()
}

// GetFieldPopulations returns all recorded field populations
func (t *DefaultFieldTracker) GetFieldPopulations() []FieldPopulation {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]FieldPopulation, len(t.populations))
	copy(result, t.populations)
	return result
}

// FieldTrackerHolder provides thread-safe access to a FieldTracker.
// Feeders embed this instead of storing a raw FieldTracker field.
type FieldTrackerHolder struct {
	mu      sync.RWMutex
	tracker FieldTracker
}

// Set stores the tracker (called by SetFieldTracker).
func (h *FieldTrackerHolder) Set(tracker FieldTracker) {
	h.mu.Lock()
	h.tracker = tracker
	h.mu.Unlock()
}

// Record is a convenience that nil-checks the tracker and calls RecordFieldPopulation.
func (h *FieldTrackerHolder) Record(fp FieldPopulation) {
	h.mu.RLock()
	t := h.tracker
	h.mu.RUnlock()
	if t != nil {
		t.RecordFieldPopulation(fp)
	}
}

// Has returns true when a tracker has been set (non-nil).
func (h *FieldTrackerHolder) Has() bool {
	h.mu.RLock()
	ok := h.tracker != nil
	h.mu.RUnlock()
	return ok
}
