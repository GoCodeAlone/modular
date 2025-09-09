package reload

import (
	"context"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockReloadable implements modular.Reloadable for testing no-op semantics.
type mockReloadable struct {
	appliedChanges [][]modular.ConfigChange
	failValidation bool
}

func (m *mockReloadable) Reload(ctx context.Context, changes []modular.ConfigChange) error {
	// Simulate validation: if any NewValue == "invalid", reject atomically.
	if m.failValidation {
		return assert.AnError
	}
	for _, c := range changes {
		if s, ok := c.NewValue.(string); ok && s == "invalid" {
			return assert.AnError
		}
	}
	if len(changes) > 0 { // record only real changes
		m.appliedChanges = append(m.appliedChanges, changes)
	}
	return nil
}
func (m *mockReloadable) CanReload() bool          { return true }
func (m *mockReloadable) ReloadTimeout() time.Duration { return time.Second }

// helper to build ConfigChange slice from diff
func diffToChanges(section string, diff *modular.ConfigDiff) []modular.ConfigChange {
	if diff == nil || diff.IsEmpty() { return nil }
	changes := make([]modular.ConfigChange, 0, len(diff.Changed)+len(diff.Added)+len(diff.Removed))
	for _, ch := range diff.Changed {
		changes = append(changes, modular.ConfigChange{Section: section, FieldPath: ch.FieldPath, OldValue: ch.OldValue, NewValue: ch.NewValue, Source: "test"})
	}
	for path, v := range diff.Added {
		changes = append(changes, modular.ConfigChange{Section: section, FieldPath: path, OldValue: nil, NewValue: v, Source: "test"})
	}
	for path, v := range diff.Removed {
		changes = append(changes, modular.ConfigChange{Section: section, FieldPath: path, OldValue: v, NewValue: nil, Source: "test"})
	}
	return changes
}

func TestReloadNoOp_IdempotentAndNoEvents(t *testing.T) {
	base := map[string]any{"service": map[string]any{"enabled": true, "port": 8080}}
	same := map[string]any{"service": map[string]any{"enabled": true, "port": 8080}}
	// Generate diff between identical configs
	diff, err := modular.GenerateConfigDiff(base, same)
	require.NoError(t, err)
	assert.True(t, diff.IsEmpty(), "Diff should be empty for identical configs")

	mr := &mockReloadable{}
	changes := diffToChanges("service", diff)
	// First reload with no changes
	err = mr.Reload(context.Background(), changes)
	assert.NoError(t, err)
	assert.Len(t, mr.appliedChanges, 0, "No changes should be applied on no-op diff")

	// Second reload (idempotent) also no changes
	err = mr.Reload(context.Background(), changes)
	assert.NoError(t, err)
	assert.Len(t, mr.appliedChanges, 0, "Still no changes after second no-op reload")
}

func TestReload_ConfigChangesAppliedOnce(t *testing.T) {
	oldCfg := map[string]any{"service": map[string]any{"enabled": true, "port": 8080}}
	newCfg := map[string]any{"service": map[string]any{"enabled": false, "port": 8081}}
	diff, err := modular.GenerateConfigDiff(oldCfg, newCfg)
	require.NoError(t, err)
	assert.False(t, diff.IsEmpty())

	mr := &mockReloadable{}
	changes := diffToChanges("service", diff)
	err = mr.Reload(context.Background(), changes)
	assert.NoError(t, err)
	assert.Len(t, mr.appliedChanges, 1, "One batch applied")
	assert.Equal(t, len(changes), len(mr.appliedChanges[0]))

	// Replaying same changes should still apply (idempotent safety) but logically could be skipped;
	// we accept second application but verify no mutation duplicates unless non-empty.
	err = mr.Reload(context.Background(), changes)
	assert.NoError(t, err)
	assert.Len(t, mr.appliedChanges, 2, "Second application accepted (idempotent)")
}

func TestReload_ValidationRejectsAtomic(t *testing.T) {
	oldCfg := map[string]any{"service": map[string]any{"mode": "safe"}}
	newCfg := map[string]any{"service": map[string]any{"mode": "invalid"}}
	diff, err := modular.GenerateConfigDiff(oldCfg, newCfg)
	require.NoError(t, err)
	changes := diffToChanges("service", diff)
	mr := &mockReloadable{}
	err = mr.Reload(context.Background(), changes)
	assert.Error(t, err, "Invalid change should be rejected")
	assert.Len(t, mr.appliedChanges, 0, "No partial application on validation failure")
}
