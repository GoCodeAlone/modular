package reload

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReloadRejectsStaticChange(t *testing.T) {
	manager := NewReloadManager([]string{"log.level", "cache.ttl"})
	base := map[string]any{"log.level": "info", "cache.ttl": 30, "server.port": 8080}
	updated := map[string]any{"log.level": "debug", "cache.ttl": 60, "server.port": 9090}
	diff, err := modular.GenerateConfigDiff(base, updated)
	require.NoError(t, err)
	r := &dynamicTestReloadable{failAt: -1}
	err = manager.ApplyDiff(context.Background(), r, "app", diff)
	assert.ErrorIs(t, err, ErrStaticFieldChange)
	assert.Len(t, r.applied, 0)
}

func TestReloadAcceptsDynamicOnly(t *testing.T) {
	manager := NewReloadManager([]string{"log.level", "cache.ttl"})
	base := map[string]any{"log.level": "info", "cache.ttl": 30, "server.port": 8080}
	updated := map[string]any{"log.level": "debug", "cache.ttl": 60, "server.port": 8080}
	diff, err := modular.GenerateConfigDiff(base, updated)
	require.NoError(t, err)
	r := &dynamicTestReloadable{failAt: -1}
	err = manager.ApplyDiff(context.Background(), r, "app", diff)
	assert.NoError(t, err)
	assert.Len(t, r.applied, 1)
	assert.Equal(t, 2, len(r.applied[0]))
}

func TestReloadMixedStaticDynamicRejected(t *testing.T) {
	manager := NewReloadManager([]string{"log.level"})
	base := map[string]any{"log.level": "info", "server.port": 8080}
	updated := map[string]any{"log.level": "debug", "server.port": 9999}
	diff, _ := modular.GenerateConfigDiff(base, updated)
	r := &dynamicTestReloadable{failAt: -1}
	err := manager.ApplyDiff(context.Background(), r, "app", diff)
	assert.ErrorIs(t, err, ErrStaticFieldChange)
	assert.Len(t, r.applied, 0)
}

