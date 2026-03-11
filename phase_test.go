package modular

import (
	"testing"
)

func TestAppPhase_String(t *testing.T) {
	tests := []struct {
		phase AppPhase
		want  string
	}{
		{PhaseCreated, "created"},
		{PhaseInitializing, "initializing"},
		{PhaseInitialized, "initialized"},
		{PhaseStarting, "starting"},
		{PhaseRunning, "running"},
		{PhaseDraining, "draining"},
		{PhaseStopping, "stopping"},
		{PhaseStopped, "stopped"},
	}
	for _, tt := range tests {
		if got := tt.phase.String(); got != tt.want {
			t.Errorf("AppPhase(%d).String() = %q, want %q", tt.phase, got, tt.want)
		}
	}
}

func TestPhaseTracking_LifecycleTransitions(t *testing.T) {
	app, err := NewApplication(WithLogger(nopLogger{}))
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}

	stdApp := app.(*StdApplication)

	if stdApp.Phase() != PhaseCreated {
		t.Errorf("expected PhaseCreated, got %v", stdApp.Phase())
	}

	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if stdApp.Phase() != PhaseInitialized {
		t.Errorf("expected PhaseInitialized after Init, got %v", stdApp.Phase())
	}

	if err := app.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if stdApp.Phase() != PhaseRunning {
		t.Errorf("expected PhaseRunning after Start, got %v", stdApp.Phase())
	}

	if err := app.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if stdApp.Phase() != PhaseStopped {
		t.Errorf("expected PhaseStopped after Stop, got %v", stdApp.Phase())
	}
}
