package modular

import (
	"testing"
)

type testDepModule struct {
	name    string
	initSeq *[]string
}

func (m *testDepModule) Name() string { return m.name }
func (m *testDepModule) Init(app Application) error {
	*m.initSeq = append(*m.initSeq, m.name)
	return nil
}

func TestWithModuleDependency_OrdersModulesCorrectly(t *testing.T) {
	seq := make([]string, 0)
	modA := &testDepModule{name: "alpha", initSeq: &seq}
	modB := &testDepModule{name: "beta", initSeq: &seq}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(modA, modB),
		WithModuleDependency("alpha", "beta"),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}

	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if len(seq) != 2 || seq[0] != "beta" || seq[1] != "alpha" {
		t.Errorf("expected init order [beta, alpha], got %v", seq)
	}
}

func TestWithModuleDependency_DetectsCycle(t *testing.T) {
	modA := &testDepModule{name: "alpha", initSeq: new([]string)}
	modB := &testDepModule{name: "beta", initSeq: new([]string)}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(modA, modB),
		WithModuleDependency("alpha", "beta"),
		WithModuleDependency("beta", "alpha"),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}

	err = app.Init()
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
	if !IsErrCircularDependency(err) {
		t.Errorf("expected ErrCircularDependency, got: %v", err)
	}
}
