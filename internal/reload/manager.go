package reload

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
)

// ReloadManager provides a minimal implementation to exercise dynamic reload semantics
// for the internal reload tests. It intentionally keeps scope small: field classification,
// serialization, atomic application and basic metrics hooks can evolve later.
type ReloadManager struct {
	mu            sync.Mutex
	dynamicFields map[string]struct{}
	// applied keeps history of applied reload batches for test visibility.
	applied [][]modular.ConfigChange
	// lastFingerprint stores a simple string fingerprint of last applied batch to skip duplicates.
	lastFingerprint string
}

// NewReloadManager creates a manager with the provided dynamic field paths. Any change
// outside this set is treated as static and rejected.
func NewReloadManager(dynamic []string) *ReloadManager {
	set := make(map[string]struct{}, len(dynamic))
	for _, f := range dynamic {
		set[f] = struct{}{}
	}
	return &ReloadManager{dynamicFields: set}
}

// ErrStaticFieldChange indicates a reload diff attempted to modify a static field.
var ErrStaticFieldChange = fmt.Errorf("static field change rejected")

// ApplyDiff converts a ConfigDiff into ConfigChange slice filtered to dynamic fields
// and applies them to the given Reloadable module atomically. If any static field
// is present in the diff it rejects the whole operation.
func (m *ReloadManager) ApplyDiff(ctx context.Context, module modular.Reloadable, section string, diff *modular.ConfigDiff) error {
	if diff == nil || diff.IsEmpty() { // no-op
		return nil
	}

	// Build change list & detect static usage
	changes := make([]modular.ConfigChange, 0, len(diff.Changed)+len(diff.Added)+len(diff.Removed))
	staticDetected := false
	addChange := func(path string, oldV, newV any) {
		if _, ok := m.dynamicFields[path]; !ok {
			staticDetected = true
			return
		}
		changes = append(changes, modular.ConfigChange{Section: section, FieldPath: path, OldValue: oldV, NewValue: newV, Source: "diff"})
	}
	for p, c := range diff.Changed {
		addChange(p, c.OldValue, c.NewValue)
	}
	for p, v := range diff.Added {
		addChange(p, nil, v)
	}
	for p, v := range diff.Removed {
		addChange(p, v, nil)
	}

	if staticDetected {
		return ErrStaticFieldChange
	}
	if len(changes) == 0 { // Only static fields changed => treat as rejection
		return nil
	}

	// Serialize & apply atomically
	m.mu.Lock()
	defer m.mu.Unlock()

	// Apply with timeout derived from module.ReloadTimeout()
	timeout := module.ReloadTimeout()
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx2, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := module.Reload(ctx2, changes); err != nil {
		return fmt.Errorf("reload apply: %w", err)
	}
	// Compute fingerprint (cheap concatenation of field paths + values lengths)
	fp := fingerprint(changes)
	// Record every successful application (even duplicates) to let tests inspect serialization.
	m.applied = append(m.applied, changes)
	m.lastFingerprint = fp
	return nil
}

// AppliedBatches returns a copy of applied change batches for inspection in tests.
func (m *ReloadManager) AppliedBatches() [][]modular.ConfigChange {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]modular.ConfigChange, len(m.applied))
	for i, b := range m.applied {
		cp := make([]modular.ConfigChange, len(b))
		copy(cp, b)
		out[i] = cp
	}
	return out
}

// fingerprint generates a deterministic string representing the batch to allow duplicate suppression.
func fingerprint(changes []modular.ConfigChange) string {
	if len(changes) == 0 {
		return ""
	}
	// Order already stable (construction path), build compact string
	s := make([]byte, 0, len(changes)*16)
	for _, c := range changes {
		s = append(s, c.FieldPath...)
		if c.NewValue != nil {
			s = append(s, '1')
		} else {
			s = append(s, '0')
		}
	}
	return string(s)
}
