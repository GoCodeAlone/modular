package eventlogger

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/modular"
)

// TestEventLoggerModule_ImplementsDrainable verifies that EventLoggerModule
// satisfies the modular.Drainable interface at compile time.
func TestEventLoggerModule_ImplementsDrainable(t *testing.T) {
	var _ modular.Drainable = (*EventLoggerModule)(nil)
}

// TestEventLoggerModule_PreStop_NotStarted verifies PreStop returns nil
// when the module has not been started.
func TestEventLoggerModule_PreStop_NotStarted(t *testing.T) {
	m := &EventLoggerModule{name: ModuleName}
	if err := m.PreStop(context.Background()); err != nil {
		t.Fatalf("PreStop on unstarted module should return nil, got: %v", err)
	}
}

// TestEventLoggerModule_PreStop_Started verifies PreStop flushes outputs
// and returns nil when the module is running.
func TestEventLoggerModule_PreStop_Started(t *testing.T) {
	m := &EventLoggerModule{
		name:    ModuleName,
		started: true,
		logger:  &testLogger{},
		outputs: []OutputTarget{},
	}

	if err := m.PreStop(context.Background()); err != nil {
		t.Fatalf("PreStop on started module should return nil, got: %v", err)
	}
}
