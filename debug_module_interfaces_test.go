package modular

import (
	"bytes"
	"os"
	"testing"
)

// localTestDbgModule distinct from any existing test module
type localTestDbgModule struct{}

func (m *localTestDbgModule) Name() string { return "test" }

// Implement minimal Module interface surface used in tests
func (m *localTestDbgModule) Init(app Application) error  { return nil }
func (m *localTestDbgModule) Start(app Application) error { return nil }
func (m *localTestDbgModule) Stop(app Application) error  { return nil }

// localNoopLogger duplicates minimal logger to avoid ordering issues
type localNoopLogger struct{}

func (n *localNoopLogger) Debug(string, ...interface{}) {}
func (n *localNoopLogger) Info(string, ...interface{})  {}
func (n *localNoopLogger) Warn(string, ...interface{})  {}
func (n *localNoopLogger) Error(string, ...interface{}) {}

// ensure it satisfies Module
var _ Module = (*localTestDbgModule)(nil)

func TestDebugModuleInterfaces_New(t *testing.T) {
	cp := NewStdConfigProvider(&minimalConfig{})
	logger := &localNoopLogger{}
	app := NewStdApplication(cp, logger).(*StdApplication)
	app.RegisterModule(&localTestDbgModule{})

	// capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	DebugModuleInterfaces(app, "test")
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	if out == "" || !bytes.Contains(buf.Bytes(), []byte("Debugging module")) {
		t.Fatalf("expected debug output, got none")
	}
}

func TestDebugModuleInterfacesNotStdApp_New(t *testing.T) {
	cp := NewStdConfigProvider(&minimalConfig{})
	logger := &localNoopLogger{}
	// Register a module on underlying std app then wrap so decorator is not *StdApplication
	underlying := NewStdApplication(cp, logger)
	underlying.RegisterModule(&localTestDbgModule{})
	base := NewBaseApplicationDecorator(underlying)
	// capture stdout for early error branch
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	DebugModuleInterfaces(base, "whatever")
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !bytes.Contains(buf.Bytes(), []byte("not a StdApplication")) {
		t.Fatalf("expected not StdApplication message")
	}
}

func TestCompareModuleInstances_New(t *testing.T) {
	m1 := &localTestDbgModule{}
	m2 := &localTestDbgModule{}
	// capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	CompareModuleInstances(m1, m2, "test")
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !bytes.Contains(buf.Bytes(), []byte("Comparing module instances")) {
		t.Fatalf("expected compare output")
	}
}
