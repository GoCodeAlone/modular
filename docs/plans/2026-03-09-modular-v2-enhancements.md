# Modular v2 Enhancements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement 12 framework enhancements to the GoCodeAlone/modular framework covering lifecycle, services, plugins, configuration, reload, and observability.

**Architecture:** All changes are in the root `modular` package except the config file watcher (new `modules/configwatcher` subpackage). The existing `Application` interface, `StdApplication` struct, and `ApplicationBuilder` are extended. New interfaces (`Drainable`, `Plugin`, `MetricsProvider`, `SecretResolver`) follow the existing optional-interface pattern. Generic service helpers use Go 1.25 type parameters.

**Tech Stack:** Go 1.25, CloudEvents SDK, fsnotify (new dependency for configwatcher)

---

### Task 1: Config-Driven Dependency Hints (`WithModuleDependency`)

**Files:**
- Modify: `builder.go` — add `dependencyHints` field, `WithModuleDependency` option
- Modify: `application.go` — merge hints into `resolveDependencies()`
- Create: `builder_dependency_test.go` — tests
- Modify: `errors.go` — add sentinel if needed

**Step 1: Write the failing test**

Create `builder_dependency_test.go`:

```go
package modular

import (
	"context"
	"testing"
)

// testDepModule is a minimal module for dependency hint testing.
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

	// Without dependency hints, alpha inits before beta (alphabetical DFS).
	// With WithModuleDependency("alpha", "beta"), beta must init first.
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
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestWithModuleDependency -count=1 -v`
Expected: FAIL — `WithModuleDependency` undefined

**Step 3: Implement**

In `builder.go`, add to `ApplicationBuilder`:
```go
dependencyHints []DependencyEdge
```

Add option function:
```go
// WithModuleDependency declares that module `from` depends on module `to`,
// injecting an edge into the dependency graph before resolution.
func WithModuleDependency(from, to string) Option {
	return func(b *ApplicationBuilder) error {
		b.dependencyHints = append(b.dependencyHints, DependencyEdge{
			From: from,
			To:   to,
			Type: EdgeTypeModule,
		})
		return nil
	}
}
```

In `Build()`, after creating the app and before registering modules, store hints on the StdApplication. Add a new field to `StdApplication`:
```go
dependencyHints []DependencyEdge
```

In `Build()`, after `app` is created, set hints:
```go
if len(b.dependencyHints) > 0 {
	if stdApp, ok := app.(*StdApplication); ok {
		stdApp.dependencyHints = b.dependencyHints
	} else if obsApp, ok := app.(*ObservableApplication); ok {
		obsApp.dependencyHints = b.dependencyHints
	}
}
```

In `resolveDependencies()` in `application.go`, after building the graph from `DependencyAware` modules (around line 1104), add:
```go
// Merge config-driven dependency hints
for _, hint := range app.dependencyHints {
	if graph[hint.From] == nil {
		graph[hint.From] = nil
	}
	graph[hint.From] = append(graph[hint.From], hint.To)
	dependencyEdges = append(dependencyEdges, hint)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run TestWithModuleDependency -count=1 -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /tmp/gca-modular
git add builder.go application.go builder_dependency_test.go
git commit -m "feat: add WithModuleDependency for config-driven dependency hints"
```

---

### Task 2: Drainable Interface (Shutdown Drain Phases)

**Files:**
- Create: `drainable.go` — interface + drain timeout option
- Modify: `application.go` — call PreStop before Stop in `Stop()`
- Modify: `builder.go` — add `WithDrainTimeout` option
- Create: `drainable_test.go` — tests

**Step 1: Write the failing test**

Create `drainable_test.go`:

```go
package modular

import (
	"context"
	"testing"
	"time"
)

type drainableModule struct {
	name       string
	preStopSeq *[]string
	stopSeq    *[]string
}

func (m *drainableModule) Name() string                        { return m.name }
func (m *drainableModule) Init(app Application) error          { return nil }
func (m *drainableModule) Start(ctx context.Context) error     { return nil }
func (m *drainableModule) PreStop(ctx context.Context) error {
	*m.preStopSeq = append(*m.preStopSeq, m.name)
	return nil
}
func (m *drainableModule) Stop(ctx context.Context) error {
	*m.stopSeq = append(*m.stopSeq, m.name)
	return nil
}

func TestDrainable_PreStopCalledBeforeStop(t *testing.T) {
	preStops := make([]string, 0)
	stops := make([]string, 0)

	mod := &drainableModule{name: "drainer", preStopSeq: &preStops, stopSeq: &stops}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(mod),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := app.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := app.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if len(preStops) != 1 || preStops[0] != "drainer" {
		t.Errorf("expected PreStop called for drainer, got %v", preStops)
	}
	if len(stops) != 1 || stops[0] != "drainer" {
		t.Errorf("expected Stop called for drainer, got %v", stops)
	}
}

func TestDrainable_WithDrainTimeout(t *testing.T) {
	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithDrainTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}

	stdApp, ok := app.(*StdApplication)
	if !ok {
		t.Skip("not a StdApplication")
	}

	if stdApp.drainTimeout != 5*time.Second {
		t.Errorf("expected drain timeout 5s, got %v", stdApp.drainTimeout)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestDrainable -count=1 -v`
Expected: FAIL — `Drainable` undefined, `WithDrainTimeout` undefined

**Step 3: Implement**

Create `drainable.go`:
```go
package modular

import (
	"context"
	"time"
)

// Drainable is an optional interface for modules that need a pre-stop drain phase.
// During shutdown, PreStop is called on all Drainable modules (reverse dependency order)
// before Stop is called on Stoppable modules. This allows modules to stop accepting
// new work and drain in-flight requests before the hard stop.
type Drainable interface {
	// PreStop initiates graceful drain before stop. The context carries the drain timeout.
	PreStop(ctx context.Context) error
}

// defaultDrainTimeout is the default timeout for the PreStop drain phase.
const defaultDrainTimeout = 15 * time.Second
```

Add `drainTimeout` field to `StdApplication` in `application.go`:
```go
drainTimeout time.Duration
```

Add `WithDrainTimeout` option in `builder.go`:
```go
// WithDrainTimeout sets the timeout for the PreStop drain phase during shutdown.
func WithDrainTimeout(d time.Duration) Option {
	return func(b *ApplicationBuilder) error {
		b.drainTimeout = d
		return nil
	}
}
```

Add `drainTimeout time.Duration` to `ApplicationBuilder`.

In `Build()`, propagate to StdApplication:
```go
if b.drainTimeout > 0 {
	if stdApp, ok := app.(*StdApplication); ok {
		stdApp.drainTimeout = b.drainTimeout
	} else if obsApp, ok := app.(*ObservableApplication); ok {
		obsApp.drainTimeout = b.drainTimeout
	}
}
```

Modify `Stop()` in `application.go` to call PreStop first:
```go
func (app *StdApplication) Stop() error {
	modules, err := app.resolveDependencies()
	if err != nil {
		return err
	}
	slices.Reverse(modules)

	// Phase 1: Drain — call PreStop on all Drainable modules
	drainTimeout := app.drainTimeout
	if drainTimeout <= 0 {
		drainTimeout = defaultDrainTimeout
	}
	drainCtx, drainCancel := context.WithTimeout(context.Background(), drainTimeout)
	defer drainCancel()

	for _, name := range modules {
		module := app.moduleRegistry[name]
		drainableModule, ok := module.(Drainable)
		if !ok {
			continue
		}
		app.logger.Info("Draining module", "module", name)
		if err := drainableModule.PreStop(drainCtx); err != nil {
			app.logger.Error("Error draining module", "module", name, "error", err)
		}
	}

	// Phase 2: Stop — call Stop on all Stoppable modules
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lastErr error
	for _, name := range modules {
		module := app.moduleRegistry[name]
		stoppableModule, ok := module.(Stoppable)
		if !ok {
			app.logger.Debug("Module does not implement Stoppable, skipping", "module", name)
			continue
		}
		app.logger.Info("Stopping module", "module", name)
		if err = stoppableModule.Stop(ctx); err != nil {
			app.logger.Error("Error stopping module", "module", name, "error", err)
			lastErr = err
		}
	}

	if app.cancel != nil {
		app.cancel()
	}
	return lastErr
}
```

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run TestDrainable -count=1 -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /tmp/gca-modular
git add drainable.go drainable_test.go application.go builder.go
git commit -m "feat: add Drainable interface with PreStop drain phase"
```

---

### Task 3: Application Phase Tracking

**Files:**
- Create: `phase.go` — AppPhase type, constants, String()
- Modify: `application.go` — add `phase` field, `Phase()` method, phase transitions
- Modify: `observer.go` — add `EventTypeAppPhaseChanged` constant
- Create: `phase_test.go` — tests

**Step 1: Write the failing test**

Create `phase_test.go`:

```go
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
	app, err := NewApplication(
		WithLogger(nopLogger{}),
	)
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
	// After Init, phase should be past initializing (at least initialized)
	phase := stdApp.Phase()
	if phase != PhaseInitialized {
		t.Errorf("expected PhaseInitialized after Init, got %v", phase)
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
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestAppPhase -count=1 -v && go test -run TestPhaseTracking -count=1 -v`
Expected: FAIL — `AppPhase` undefined

**Step 3: Implement**

Create `phase.go`:
```go
package modular

import "sync/atomic"

// AppPhase represents the current lifecycle phase of the application.
type AppPhase int32

const (
	PhaseCreated      AppPhase = iota
	PhaseInitializing
	PhaseInitialized
	PhaseStarting
	PhaseRunning
	PhaseDraining
	PhaseStopping
	PhaseStopped
)

func (p AppPhase) String() string {
	switch p {
	case PhaseCreated:
		return "created"
	case PhaseInitializing:
		return "initializing"
	case PhaseInitialized:
		return "initialized"
	case PhaseStarting:
		return "starting"
	case PhaseRunning:
		return "running"
	case PhaseDraining:
		return "draining"
	case PhaseStopping:
		return "stopping"
	case PhaseStopped:
		return "stopped"
	default:
		return "unknown"
	}
}
```

Add `phase atomic.Int32` field to `StdApplication`. Add `Phase()` method:
```go
func (app *StdApplication) Phase() AppPhase {
	return AppPhase(app.phase.Load())
}

func (app *StdApplication) setPhase(p AppPhase) {
	app.phase.Store(int32(p))
}
```

Add `EventTypeAppPhaseChanged` to `observer.go`:
```go
EventTypeAppPhaseChanged = "com.modular.application.phase.changed"
```

In `InitWithApp()`, wrap with phase transitions:
```go
app.setPhase(PhaseInitializing)
// ... existing init logic ...
app.setPhase(PhaseInitialized)
```

In `Start()`:
```go
app.setPhase(PhaseStarting)
// ... existing start logic ...
app.setPhase(PhaseRunning)
```

In `Stop()`:
```go
app.setPhase(PhaseDraining)
// ... PreStop phase ...
app.setPhase(PhaseStopping)
// ... Stop phase ...
app.setPhase(PhaseStopped)
```

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run "TestAppPhase|TestPhaseTracking" -count=1 -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /tmp/gca-modular
git add phase.go phase_test.go application.go observer.go
git commit -m "feat: add application phase tracking with lifecycle transitions"
```

---

### Task 4: Parallel Init at Same Topological Depth

**Files:**
- Modify: `builder.go` — add `WithParallelInit` option
- Modify: `application.go` — parallel init logic using `errgroup`
- Create: `parallel_init_test.go` — tests

**Step 1: Write the failing test**

Create `parallel_init_test.go`:

```go
package modular

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type parallelInitModule struct {
	name      string
	deps      []string
	initDelay time.Duration
	initCount *atomic.Int32
	maxPar    *atomic.Int32
	curPar    *atomic.Int32
}

func (m *parallelInitModule) Name() string        { return m.name }
func (m *parallelInitModule) Dependencies() []string { return m.deps }
func (m *parallelInitModule) Init(app Application) error {
	cur := m.curPar.Add(1)
	defer m.curPar.Add(-1)

	// Track max concurrency
	for {
		old := m.maxPar.Load()
		if cur <= old || m.maxPar.CompareAndSwap(old, cur) {
			break
		}
	}

	m.initCount.Add(1)
	time.Sleep(m.initDelay)
	return nil
}

func TestWithParallelInit_ConcurrentSameDepth(t *testing.T) {
	var initCount, maxPar, curPar atomic.Int32

	// Three independent modules (no deps) — should init concurrently
	modA := &parallelInitModule{name: "a", initDelay: 50 * time.Millisecond, initCount: &initCount, maxPar: &maxPar, curPar: &curPar}
	modB := &parallelInitModule{name: "b", initDelay: 50 * time.Millisecond, initCount: &initCount, maxPar: &maxPar, curPar: &curPar}
	modC := &parallelInitModule{name: "c", initDelay: 50 * time.Millisecond, initCount: &initCount, maxPar: &maxPar, curPar: &curPar}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(modA, modB, modC),
		WithParallelInit(),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}

	start := time.Now()
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	elapsed := time.Since(start)

	if initCount.Load() != 3 {
		t.Errorf("expected 3 inits, got %d", initCount.Load())
	}
	if maxPar.Load() < 2 {
		t.Errorf("expected at least 2 concurrent inits, got max %d", maxPar.Load())
	}
	// Should complete faster than 3 * 50ms sequential
	if elapsed > 120*time.Millisecond {
		t.Errorf("expected parallel init to be faster, took %v", elapsed)
	}
}

func TestWithParallelInit_RespectsDepOrder(t *testing.T) {
	var mu sync.Mutex
	order := make([]string, 0)

	makeModule := func(name string, deps []string) *simpleOrderModule {
		return &simpleOrderModule{name: name, deps: deps, order: &order, mu: &mu}
	}

	// dep → a, dep → b (a and b can be parallel, dep must be first)
	modDep := makeModule("dep", nil)
	modA := makeModule("a", []string{"dep"})
	modB := makeModule("b", []string{"dep"})

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(modDep, modA, modB),
		WithParallelInit(),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 || order[0] != "dep" {
		t.Errorf("expected dep first, got order %v", order)
	}
}

type simpleOrderModule struct {
	name  string
	deps  []string
	order *[]string
	mu    *sync.Mutex
}

func (m *simpleOrderModule) Name() string           { return m.name }
func (m *simpleOrderModule) Dependencies() []string  { return m.deps }
func (m *simpleOrderModule) Init(app Application) error {
	m.mu.Lock()
	*m.order = append(*m.order, m.name)
	m.mu.Unlock()
	return nil
}
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestWithParallelInit -count=1 -v`
Expected: FAIL — `WithParallelInit` undefined

**Step 3: Implement**

Add `parallelInit bool` field to `ApplicationBuilder` and `StdApplication`.

Add builder option:
```go
// WithParallelInit enables concurrent initialization of modules at the same
// topological depth in the dependency graph. Disabled by default.
func WithParallelInit() Option {
	return func(b *ApplicationBuilder) error {
		b.parallelInit = true
		return nil
	}
}
```

Propagate in `Build()` similar to other fields.

In `application.go`, add a method to compute topological depth levels:
```go
// computeDepthLevels groups modules by their topological depth.
// Level 0 has no dependencies, level 1 depends only on level 0, etc.
func (app *StdApplication) computeDepthLevels(order []string) [][]string {
	depth := make(map[string]int)
	graph := make(map[string][]string)

	// Rebuild graph for depth calculation
	for _, name := range order {
		module := app.moduleRegistry[name]
		if depAware, ok := module.(DependencyAware); ok {
			graph[name] = depAware.Dependencies()
		}
		// Include config-driven hints
		for _, hint := range app.dependencyHints {
			if hint.From == name {
				graph[name] = append(graph[name], hint.To)
			}
		}
	}

	// Compute depths
	var computeDepth func(string) int
	computeDepth = func(name string) int {
		if d, ok := depth[name]; ok {
			return d
		}
		maxDep := 0
		for _, dep := range graph[name] {
			if d := computeDepth(dep) + 1; d > maxDep {
				maxDep = d
			}
		}
		depth[name] = maxDep
		return maxDep
	}

	for _, name := range order {
		computeDepth(name)
	}

	// Group by depth
	maxDepth := 0
	for _, d := range depth {
		if d > maxDepth {
			maxDepth = d
		}
	}

	levels := make([][]string, maxDepth+1)
	for _, name := range order {
		d := depth[name]
		levels[d] = append(levels[d], name)
	}
	return levels
}
```

Modify `InitWithApp` to use parallel init when enabled. Replace the sequential init loop with:
```go
if app.parallelInit {
	levels := app.computeDepthLevels(moduleOrder)
	for _, level := range levels {
		if len(level) == 1 {
			// Single module — init sequentially (no goroutine overhead)
			if err := app.initModule(appToPass, level[0]); err != nil {
				errs = append(errs, err)
			}
		} else {
			// Multiple modules at same depth — init concurrently
			var levelErrs []error
			var mu sync.Mutex
			var wg sync.WaitGroup
			for _, moduleName := range level {
				wg.Add(1)
				go func(name string) {
					defer wg.Done()
					if err := app.initModule(appToPass, name); err != nil {
						mu.Lock()
						levelErrs = append(levelErrs, err)
						mu.Unlock()
					}
				}(moduleName)
			}
			wg.Wait()
			errs = append(errs, levelErrs...)
		}
	}
} else {
	// Sequential init (existing behavior)
	for _, moduleName := range moduleOrder {
		if err := app.initModule(appToPass, moduleName); err != nil {
			errs = append(errs, err)
		}
	}
}
```

Extract the per-module init logic into a helper:
```go
func (app *StdApplication) initModule(appToPass Application, moduleName string) error {
	var err error
	module := app.moduleRegistry[moduleName]

	if _, ok := module.(ServiceAware); ok {
		app.moduleRegistry[moduleName], err = app.injectServices(module)
		if err != nil {
			return fmt.Errorf("failed to inject services for module '%s': %w", moduleName, err)
		}
		module = app.moduleRegistry[moduleName]
	}

	if app.enhancedSvcRegistry != nil {
		app.enhancedSvcRegistry.SetCurrentModule(module)
	}

	if err = module.Init(appToPass); err != nil {
		if app.enhancedSvcRegistry != nil {
			app.enhancedSvcRegistry.ClearCurrentModule()
		}
		return fmt.Errorf("module '%s' failed to initialize: %w", moduleName, err)
	}

	if svcAware, ok := module.(ServiceAware); ok {
		for _, svc := range svcAware.ProvidesServices() {
			if err = app.RegisterService(svc.Name, svc.Instance); err != nil {
				if app.enhancedSvcRegistry != nil {
					app.enhancedSvcRegistry.ClearCurrentModule()
				}
				return fmt.Errorf("module '%s' failed to register service '%s': %w", moduleName, svc.Name, err)
			}
		}
	}

	if app.enhancedSvcRegistry != nil {
		app.enhancedSvcRegistry.ClearCurrentModule()
	}

	app.logger.Info(fmt.Sprintf("Initialized module %s of type %T", moduleName, app.moduleRegistry[moduleName]))
	return nil
}
```

**Note:** When parallel init is enabled, `SetCurrentModule`/`ClearCurrentModule` need mutex protection. Add a mutex to the init path or guard the enhanced registry calls.

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run TestWithParallelInit -count=1 -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd /tmp/gca-modular && go test ./... -count=1`
Expected: All existing tests pass

**Step 6: Commit**

```bash
cd /tmp/gca-modular
git add builder.go application.go parallel_init_test.go
git commit -m "feat: add WithParallelInit for concurrent module initialization"
```

---

### Task 5: Type-Safe Service Helpers (Generics)

**Files:**
- Create: `service_typed.go` — generic helper functions
- Create: `service_typed_test.go` — tests

**Step 1: Write the failing test**

Create `service_typed_test.go`:

```go
package modular

import (
	"testing"
)

type testService struct {
	Value string
}

func TestRegisterTypedService_and_GetTypedService(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(&struct{}{}), nopLogger{})

	svc := &testService{Value: "hello"}
	if err := RegisterTypedService[*testService](app, "test.svc", svc); err != nil {
		t.Fatalf("RegisterTypedService: %v", err)
	}

	got, err := GetTypedService[*testService](app, "test.svc")
	if err != nil {
		t.Fatalf("GetTypedService: %v", err)
	}
	if got.Value != "hello" {
		t.Errorf("expected hello, got %s", got.Value)
	}
}

func TestGetTypedService_WrongType(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(&struct{}{}), nopLogger{})
	_ = RegisterTypedService[string](app, "str.svc", "hello")

	_, err := GetTypedService[int](app, "str.svc")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}

func TestGetTypedService_NotFound(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(&struct{}{}), nopLogger{})

	_, err := GetTypedService[string](app, "missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestRegisterTypedService -count=1 -v && go test -run TestGetTypedService -count=1 -v`
Expected: FAIL — `RegisterTypedService` undefined

**Step 3: Implement**

Create `service_typed.go`:

```go
package modular

import "fmt"

// RegisterTypedService registers a service with compile-time type safety.
// This is a package-level helper that wraps Application.RegisterService.
func RegisterTypedService[T any](app Application, name string, svc T) error {
	return app.RegisterService(name, svc)
}

// GetTypedService retrieves a service with compile-time type safety.
// Returns the zero value of T and an error if the service is not found
// or cannot be cast to the expected type.
func GetTypedService[T any](app Application, name string) (T, error) {
	var zero T
	svcRegistry := app.SvcRegistry()
	raw, exists := svcRegistry[name]
	if !exists {
		return zero, fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}
	typed, ok := raw.(T)
	if !ok {
		return zero, fmt.Errorf("%w: service %q is %T, want %T", ErrServiceWrongType, name, raw, zero)
	}
	return typed, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run "TestRegisterTypedService|TestGetTypedService" -count=1 -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /tmp/gca-modular
git add service_typed.go service_typed_test.go
git commit -m "feat: add RegisterTypedService/GetTypedService generic helpers"
```

---

### Task 6: Service Readiness Events & OnServiceReady

**Files:**
- Modify: `service.go` — add `OnServiceReady` method to `EnhancedServiceRegistry`
- Create: `service_readiness_test.go` — tests

**Step 1: Write the failing test**

Create `service_readiness_test.go`:

```go
package modular

import (
	"sync/atomic"
	"testing"
)

func TestOnServiceReady_AlreadyRegistered(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	registry.RegisterService("db", "postgres-conn")

	var called atomic.Bool
	registry.OnServiceReady("db", func(svc any) {
		called.Store(true)
		if svc != "postgres-conn" {
			t.Errorf("expected postgres-conn, got %v", svc)
		}
	})

	if !called.Load() {
		t.Error("callback should have been called immediately")
	}
}

func TestOnServiceReady_DeferredUntilRegistration(t *testing.T) {
	registry := NewEnhancedServiceRegistry()

	var called atomic.Bool
	var receivedSvc any
	registry.OnServiceReady("db", func(svc any) {
		called.Store(true)
		receivedSvc = svc
	})

	if called.Load() {
		t.Error("callback should not have been called yet")
	}

	registry.RegisterService("db", "postgres-conn")

	if !called.Load() {
		t.Error("callback should have been called after registration")
	}
	if receivedSvc != "postgres-conn" {
		t.Errorf("expected postgres-conn, got %v", receivedSvc)
	}
}

func TestOnServiceReady_MultipleCallbacks(t *testing.T) {
	registry := NewEnhancedServiceRegistry()

	var count atomic.Int32
	registry.OnServiceReady("cache", func(svc any) { count.Add(1) })
	registry.OnServiceReady("cache", func(svc any) { count.Add(1) })

	registry.RegisterService("cache", "redis")

	if count.Load() != 2 {
		t.Errorf("expected 2 callbacks, got %d", count.Load())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestOnServiceReady -count=1 -v`
Expected: FAIL — `OnServiceReady` undefined

**Step 3: Implement**

Add to `EnhancedServiceRegistry`:
```go
// readyCallbacks maps service names to pending callbacks.
readyCallbacks map[string][]func(any)
```

Initialize in `NewEnhancedServiceRegistry`:
```go
readyCallbacks: make(map[string][]func(any)),
```

Add the method:
```go
// OnServiceReady registers a callback that fires when the named service is registered.
// If the service is already registered, the callback fires immediately.
func (r *EnhancedServiceRegistry) OnServiceReady(name string, callback func(any)) {
	if entry, exists := r.services[name]; exists {
		callback(entry.Service)
		return
	}
	r.readyCallbacks[name] = append(r.readyCallbacks[name], callback)
}
```

Modify `RegisterService` to fire pending callbacks after registration:
```go
// After r.services[actualName] = entry, add:
// Fire readiness callbacks for the original name and the actual name.
for _, cbName := range []string{originalName, actualName} {
	if callbacks, ok := r.readyCallbacks[cbName]; ok {
		for _, cb := range callbacks {
			cb(service)
		}
		delete(r.readyCallbacks, cbName)
	}
}
```

Note: Use `originalName` as the variable name for the first parameter to `RegisterService` (it's called `name` in the current code — rename to `originalName` for clarity, or just use `name`).

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run TestOnServiceReady -count=1 -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /tmp/gca-modular
git add service.go service_readiness_test.go
git commit -m "feat: add OnServiceReady callback for service readiness events"
```

---

### Task 7: Plugin Interface & WithPlugins

**Files:**
- Create: `plugin.go` — Plugin, PluginWithHooks, PluginWithServices interfaces + ServiceDefinition
- Modify: `builder.go` — add `WithPlugins` option
- Create: `plugin_test.go` — tests

**Step 1: Write the failing test**

Create `plugin_test.go`:

```go
package modular

import (
	"testing"
)

type testPlugin struct {
	modules  []Module
	services []ServiceDefinition
	hookRan  bool
}

func (p *testPlugin) Name() string      { return "test-plugin" }
func (p *testPlugin) Modules() []Module { return p.modules }
func (p *testPlugin) Services() []ServiceDefinition { return p.services }
func (p *testPlugin) InitHooks() []func(Application) error {
	return []func(Application) error{
		func(app Application) error {
			p.hookRan = true
			return nil
		},
	}
}

type pluginModule struct {
	name string
	initialized bool
}

func (m *pluginModule) Name() string              { return m.name }
func (m *pluginModule) Init(app Application) error { m.initialized = true; return nil }

func TestWithPlugins_RegistersModulesAndServices(t *testing.T) {
	mod := &pluginModule{name: "plugin-mod"}
	plugin := &testPlugin{
		modules:  []Module{mod},
		services: []ServiceDefinition{{Name: "plugin.svc", Service: "hello"}},
	}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithPlugins(plugin),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !mod.initialized {
		t.Error("plugin module should have been initialized")
	}
	if !plugin.hookRan {
		t.Error("plugin hook should have run")
	}

	svc, err := GetTypedService[string](app, "plugin.svc")
	if err != nil {
		t.Fatalf("GetTypedService: %v", err)
	}
	if svc != "hello" {
		t.Errorf("expected hello, got %s", svc)
	}
}

// Test a simple plugin (no hooks, no services)
type simplePlugin struct {
	modules []Module
}

func (p *simplePlugin) Name() string      { return "simple" }
func (p *simplePlugin) Modules() []Module { return p.modules }

func TestWithPlugins_SimplePlugin(t *testing.T) {
	mod := &pluginModule{name: "simple-mod"}
	plugin := &simplePlugin{modules: []Module{mod}}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithPlugins(plugin),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !mod.initialized {
		t.Error("plugin module should have been initialized")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestWithPlugins -count=1 -v`
Expected: FAIL — `Plugin` undefined

**Step 3: Implement**

Create `plugin.go`:
```go
package modular

// Plugin is the minimal interface for a plugin bundle that provides modules.
type Plugin interface {
	Name() string
	Modules() []Module
}

// PluginWithHooks extends Plugin with initialization hooks.
type PluginWithHooks interface {
	Plugin
	InitHooks() []func(Application) error
}

// PluginWithServices extends Plugin with service definitions.
type PluginWithServices interface {
	Plugin
	Services() []ServiceDefinition
}

// ServiceDefinition describes a service provided by a plugin.
type ServiceDefinition struct {
	Name    string
	Service any
}
```

Add `plugins []Plugin` to `ApplicationBuilder`. Add option:
```go
// WithPlugins registers plugins with the application. Each plugin's modules
// are registered, hooks are added as config-loaded hooks, and services are
// registered before module init.
func WithPlugins(plugins ...Plugin) Option {
	return func(b *ApplicationBuilder) error {
		b.plugins = append(b.plugins, plugins...)
		return nil
	}
}
```

In `Build()`, after creating the app, process plugins:
```go
for _, plugin := range b.plugins {
	// Register plugin modules
	for _, mod := range plugin.Modules() {
		app.RegisterModule(mod)
	}

	// Register plugin services
	if withSvc, ok := plugin.(PluginWithServices); ok {
		for _, svcDef := range withSvc.Services() {
			if err := app.RegisterService(svcDef.Name, svcDef.Service); err != nil {
				return nil, fmt.Errorf("plugin %q service %q: %w", plugin.Name(), svcDef.Name, err)
			}
		}
	}

	// Register plugin hooks
	if withHooks, ok := plugin.(PluginWithHooks); ok {
		for _, hook := range withHooks.InitHooks() {
			app.OnConfigLoaded(hook)
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run TestWithPlugins -count=1 -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /tmp/gca-modular
git add plugin.go plugin_test.go builder.go
git commit -m "feat: add Plugin interface with WithPlugins builder option"
```

---

### Task 8: ReloadOrchestrator Integration (`WithDynamicReload`)

**Files:**
- Modify: `builder.go` — add `WithDynamicReload` option
- Modify: `application.go` — wire orchestrator into Start/Stop, expose `RequestReload`
- Create: `reload_integration_test.go` — tests

**Step 1: Write the failing test**

Create `reload_integration_test.go`:

```go
package modular

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type reloadableTestModule struct {
	name        string
	reloadCount atomic.Int32
}

func (m *reloadableTestModule) Name() string              { return m.name }
func (m *reloadableTestModule) Init(app Application) error { return nil }
func (m *reloadableTestModule) CanReload() bool            { return true }
func (m *reloadableTestModule) ReloadTimeout() time.Duration { return 5 * time.Second }
func (m *reloadableTestModule) Reload(ctx context.Context, changes []ConfigChange) error {
	m.reloadCount.Add(1)
	return nil
}

func TestWithDynamicReload_WiresOrchestrator(t *testing.T) {
	mod := &reloadableTestModule{name: "hot-mod"}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(mod),
		WithDynamicReload(),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}

	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := app.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	stdApp := app.(*StdApplication)

	// Request a reload
	diff := ConfigDiff{
		Changed: map[string]FieldChange{
			"key": {OldValue: "old", NewValue: "new", FieldPath: "key", ChangeType: ChangeModified},
		},
		DiffID: "test-diff",
	}
	err = stdApp.RequestReload(context.Background(), ReloadManual, diff)
	if err != nil {
		t.Fatalf("RequestReload: %v", err)
	}

	// Wait for reload to process
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mod.reloadCount.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if mod.reloadCount.Load() == 0 {
		t.Error("expected module to be reloaded")
	}

	if err := app.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestWithDynamicReload -count=1 -v`
Expected: FAIL — `WithDynamicReload` undefined

**Step 3: Implement**

Add `dynamicReload bool` to `ApplicationBuilder`.
Add `reloadOrchestrator *ReloadOrchestrator` to `StdApplication`.

Builder option:
```go
// WithDynamicReload enables the ReloadOrchestrator, wiring it into the
// application lifecycle. Reloadable modules are auto-registered after Init,
// and the orchestrator starts/stops with the application.
func WithDynamicReload() Option {
	return func(b *ApplicationBuilder) error {
		b.dynamicReload = true
		return nil
	}
}
```

In `Build()`, propagate:
```go
if b.dynamicReload {
	if stdApp, ok := app.(*StdApplication); ok {
		stdApp.dynamicReload = true
	} else if obsApp, ok := app.(*ObservableApplication); ok {
		obsApp.dynamicReload = true
	}
}
```

Add `dynamicReload bool` field to `StdApplication`.

In `InitWithApp`, after all modules are initialized (before marking initialized), register reloadables:
```go
if app.dynamicReload {
	var subject Subject
	if obsApp, ok := appToPass.(*ObservableApplication); ok {
		subject = obsApp
	}
	app.reloadOrchestrator = NewReloadOrchestrator(app.logger, subject)
	for name, module := range app.moduleRegistry {
		if reloadable, ok := module.(Reloadable); ok {
			app.reloadOrchestrator.RegisterReloadable(name, reloadable)
		}
	}
}
```

In `Start()`, after starting all modules:
```go
if app.reloadOrchestrator != nil {
	app.reloadOrchestrator.Start(ctx)
}
```

In `Stop()`, before draining:
```go
if app.reloadOrchestrator != nil {
	app.reloadOrchestrator.Stop()
}
```

Add `RequestReload` method:
```go
// RequestReload enqueues a reload request. Only available when WithDynamicReload is enabled.
func (app *StdApplication) RequestReload(ctx context.Context, trigger ReloadTrigger, diff ConfigDiff) error {
	if app.reloadOrchestrator == nil {
		return fmt.Errorf("dynamic reload not enabled")
	}
	return app.reloadOrchestrator.RequestReload(ctx, trigger, diff)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run TestWithDynamicReload -count=1 -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /tmp/gca-modular
git add builder.go application.go reload_integration_test.go
git commit -m "feat: add WithDynamicReload to wire ReloadOrchestrator into app lifecycle"
```

---

### Task 9: Secret Resolution Hooks

**Files:**
- Create: `secret_resolver.go` — SecretResolver interface + ExpandSecrets utility
- Create: `secret_resolver_test.go` — tests

**Step 1: Write the failing test**

Create `secret_resolver_test.go`:

```go
package modular

import (
	"context"
	"strings"
	"testing"
)

type mockResolver struct {
	prefix string
	values map[string]string
}

func (r *mockResolver) CanResolve(ref string) bool {
	return strings.HasPrefix(ref, r.prefix+":")
}

func (r *mockResolver) ResolveSecret(ctx context.Context, ref string) (string, error) {
	key := strings.TrimPrefix(ref, r.prefix+":")
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", ErrServiceNotFound
}

func TestExpandSecrets_ResolvesRefs(t *testing.T) {
	resolver := &mockResolver{
		prefix: "vault",
		values: map[string]string{
			"secret/db-pass": "s3cret",
		},
	}

	config := map[string]any{
		"host":     "localhost",
		"password": "${vault:secret/db-pass}",
		"nested": map[string]any{
			"key": "${vault:secret/db-pass}",
		},
	}

	err := ExpandSecrets(context.Background(), config, resolver)
	if err != nil {
		t.Fatalf("ExpandSecrets: %v", err)
	}

	if config["password"] != "s3cret" {
		t.Errorf("expected s3cret, got %v", config["password"])
	}
	nested := config["nested"].(map[string]any)
	if nested["key"] != "s3cret" {
		t.Errorf("expected nested s3cret, got %v", nested["key"])
	}
}

func TestExpandSecrets_SkipsNonRefs(t *testing.T) {
	config := map[string]any{
		"host": "localhost",
		"port": 5432,
	}

	err := ExpandSecrets(context.Background(), config)
	if err != nil {
		t.Fatalf("ExpandSecrets: %v", err)
	}

	if config["host"] != "localhost" {
		t.Errorf("expected localhost, got %v", config["host"])
	}
}

func TestExpandSecrets_NoMatchingResolver(t *testing.T) {
	config := map[string]any{
		"password": "${aws:secret/key}",
	}

	resolver := &mockResolver{prefix: "vault", values: map[string]string{}}

	// No matching resolver — value should remain unchanged
	err := ExpandSecrets(context.Background(), config, resolver)
	if err != nil {
		t.Fatalf("ExpandSecrets: %v", err)
	}
	if config["password"] != "${aws:secret/key}" {
		t.Errorf("expected unchanged ref, got %v", config["password"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestExpandSecrets -count=1 -v`
Expected: FAIL — `SecretResolver` undefined

**Step 3: Implement**

Create `secret_resolver.go`:

```go
package modular

import (
	"context"
	"fmt"
	"regexp"
)

// SecretResolver resolves secret references in configuration values.
// Implementations connect to secret stores (Vault, AWS Secrets Manager, etc.)
type SecretResolver interface {
	// ResolveSecret resolves a secret reference string to its actual value.
	ResolveSecret(ctx context.Context, ref string) (string, error)

	// CanResolve reports whether this resolver handles the given reference.
	CanResolve(ref string) bool
}

// secretRefPattern matches ${prefix:path} patterns in config values.
var secretRefPattern = regexp.MustCompile(`^\$\{([^:}]+:[^}]+)\}$`)

// ExpandSecrets walks a config map and replaces string values matching
// ${prefix:path} with the resolved secret value. It recurses into nested
// maps. Values that don't match or have no matching resolver are left unchanged.
func ExpandSecrets(ctx context.Context, config map[string]any, resolvers ...SecretResolver) error {
	for key, val := range config {
		switch v := val.(type) {
		case string:
			resolved, err := resolveSecretString(ctx, v, resolvers)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", key, err)
			}
			config[key] = resolved
		case map[string]any:
			if err := ExpandSecrets(ctx, v, resolvers...); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveSecretString(ctx context.Context, val string, resolvers []SecretResolver) (string, error) {
	match := secretRefPattern.FindStringSubmatch(val)
	if match == nil {
		return val, nil
	}
	ref := match[1]
	for _, r := range resolvers {
		if r.CanResolve(ref) {
			return r.ResolveSecret(ctx, ref)
		}
	}
	// No matching resolver — return unchanged
	return val, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run TestExpandSecrets -count=1 -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /tmp/gca-modular
git add secret_resolver.go secret_resolver_test.go
git commit -m "feat: add SecretResolver interface and ExpandSecrets utility"
```

---

### Task 10: Config File Watcher Module

**Files:**
- Create: `modules/configwatcher/configwatcher.go` — module implementation
- Create: `modules/configwatcher/configwatcher_test.go` — tests
- Modify: `go.mod` — add `github.com/fsnotify/fsnotify` dependency

**Step 1: Add fsnotify dependency**

Run: `cd /tmp/gca-modular && go get github.com/fsnotify/fsnotify`

**Step 2: Write the test**

Create `modules/configwatcher/configwatcher_test.go`:

```go
package configwatcher

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigWatcher_DetectsFileChange(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("key: value1"), 0644); err != nil {
		t.Fatal(err)
	}

	var changeCount atomic.Int32
	w := New(
		WithPaths(cfgFile),
		WithDebounce(50*time.Millisecond),
		WithOnChange(func(paths []string) {
			changeCount.Add(1)
		}),
	)

	if err := w.startWatching(); err != nil {
		t.Fatalf("startWatching: %v", err)
	}
	defer w.stopWatching()

	// Modify the file
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(cfgFile, []byte("key: value2"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce + processing
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if changeCount.Load() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if changeCount.Load() == 0 {
		t.Error("expected at least one change notification")
	}
}

func TestConfigWatcher_Debounces(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	var changeCount atomic.Int32
	w := New(
		WithPaths(cfgFile),
		WithDebounce(200*time.Millisecond),
		WithOnChange(func(paths []string) {
			changeCount.Add(1)
		}),
	)

	if err := w.startWatching(); err != nil {
		t.Fatalf("startWatching: %v", err)
	}
	defer w.stopWatching()

	time.Sleep(100 * time.Millisecond)
	// Rapid-fire writes
	for i := 0; i < 5; i++ {
		os.WriteFile(cfgFile, []byte("v"+string(rune('2'+i))), 0644)
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for debounce
	time.Sleep(500 * time.Millisecond)

	if changeCount.Load() > 2 {
		t.Errorf("expected debounced to ~1-2 calls, got %d", changeCount.Load())
	}
}
```

**Step 3: Implement**

Create `modules/configwatcher/configwatcher.go`:

```go
// Package configwatcher provides a module that watches configuration files
// for changes and triggers reload via a callback.
package configwatcher

import (
	"context"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher watches config files and calls OnChange when modifications are detected.
type ConfigWatcher struct {
	paths     []string
	debounce  time.Duration
	onChange  func(paths []string)
	watcher   *fsnotify.Watcher
	stopCh    chan struct{}
	stopOnce  sync.Once
}

// Option configures a ConfigWatcher.
type Option func(*ConfigWatcher)

// WithPaths sets the file paths to watch.
func WithPaths(paths ...string) Option {
	return func(w *ConfigWatcher) {
		w.paths = append(w.paths, paths...)
	}
}

// WithDebounce sets the debounce duration for file change events.
func WithDebounce(d time.Duration) Option {
	return func(w *ConfigWatcher) {
		w.debounce = d
	}
}

// WithOnChange sets the callback invoked when watched files change.
func WithOnChange(fn func(paths []string)) Option {
	return func(w *ConfigWatcher) {
		w.onChange = fn
	}
}

// New creates a new ConfigWatcher with the given options.
func New(opts ...Option) *ConfigWatcher {
	w := &ConfigWatcher{
		debounce: 500 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Name returns the module name.
func (w *ConfigWatcher) Name() string { return "configwatcher" }

// Init is a no-op for the config watcher module.
func (w *ConfigWatcher) Init(_ interface{ Logger() interface{ Info(string, ...any) } }) error {
	return nil
}

// Start begins watching the configured paths.
func (w *ConfigWatcher) Start(ctx context.Context) error {
	if err := w.startWatching(); err != nil {
		return err
	}
	go func() {
		select {
		case <-ctx.Done():
			w.stopWatching()
		case <-w.stopCh:
		}
	}()
	return nil
}

// Stop stops the file watcher.
func (w *ConfigWatcher) Stop(_ context.Context) error {
	w.stopWatching()
	return nil
}

func (w *ConfigWatcher) startWatching() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.watcher = watcher

	for _, path := range w.paths {
		if err := watcher.Add(path); err != nil {
			watcher.Close()
			return err
		}
	}

	go w.eventLoop()
	return nil
}

func (w *ConfigWatcher) stopWatching() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
		if w.watcher != nil {
			w.watcher.Close()
		}
	})
}

func (w *ConfigWatcher) eventLoop() {
	var timer *time.Timer
	changedPaths := make(map[string]struct{})

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				changedPaths[event.Name] = struct{}{}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(w.debounce, func() {
					if w.onChange != nil {
						paths := make([]string, 0, len(changedPaths))
						for p := range changedPaths {
							paths = append(paths, p)
						}
						changedPaths = make(map[string]struct{})
						w.onChange(paths)
					}
				})
			}
		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
		case <-w.stopCh:
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test ./modules/configwatcher/... -count=1 -v`
Expected: PASS

**Step 5: Run `go mod tidy`**

Run: `cd /tmp/gca-modular && go mod tidy`

**Step 6: Commit**

```bash
cd /tmp/gca-modular
git add modules/configwatcher/ go.mod go.sum
git commit -m "feat: add configwatcher module with fsnotify file watching"
```

---

### Task 11: Slog Adapter

**Files:**
- Create: `slog_adapter.go` — SlogAdapter implementation
- Create: `slog_adapter_test.go` — tests

**Step 1: Write the failing test**

Create `slog_adapter_test.go`:

```go
package modular

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSlogAdapter_ImplementsLogger(t *testing.T) {
	var _ Logger = (*SlogAdapter)(nil) // compile-time check
}

func TestSlogAdapter_DelegatesToSlog(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	adapter := NewSlogAdapter(logger)
	adapter.Info("test info", "key", "value")
	adapter.Error("test error", "err", "fail")
	adapter.Warn("test warn")
	adapter.Debug("test debug")

	output := buf.String()
	if !strings.Contains(output, "test info") {
		t.Error("expected info message in output")
	}
	if !strings.Contains(output, "test error") {
		t.Error("expected error message in output")
	}
	if !strings.Contains(output, "test warn") {
		t.Error("expected warn message in output")
	}
	if !strings.Contains(output, "test debug") {
		t.Error("expected debug message in output")
	}
}

func TestSlogAdapter_With(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	adapter := NewSlogAdapter(logger).With("module", "test")
	adapter.Info("with test")

	output := buf.String()
	if !strings.Contains(output, "module=test") {
		t.Errorf("expected module=test in output, got: %s", output)
	}
}

func TestSlogAdapter_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	adapter := NewSlogAdapter(logger).WithGroup("mygroup")
	adapter.Info("group test", "key", "val")

	output := buf.String()
	if !strings.Contains(output, "mygroup") {
		t.Errorf("expected mygroup in output, got: %s", output)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestSlogAdapter -count=1 -v`
Expected: FAIL — `SlogAdapter` undefined

**Step 3: Implement**

Create `slog_adapter.go`:

```go
package modular

import "log/slog"

// SlogAdapter wraps a *slog.Logger to implement the Logger interface.
// This allows using Go's standard structured logger with the modular framework.
type SlogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a new SlogAdapter wrapping the given slog.Logger.
func NewSlogAdapter(l *slog.Logger) *SlogAdapter {
	return &SlogAdapter{logger: l}
}

// Info logs at info level.
func (a *SlogAdapter) Info(msg string, args ...any) { a.logger.Info(msg, args...) }

// Error logs at error level.
func (a *SlogAdapter) Error(msg string, args ...any) { a.logger.Error(msg, args...) }

// Warn logs at warn level.
func (a *SlogAdapter) Warn(msg string, args ...any) { a.logger.Warn(msg, args...) }

// Debug logs at debug level.
func (a *SlogAdapter) Debug(msg string, args ...any) { a.logger.Debug(msg, args...) }

// With returns a new SlogAdapter with the given key-value pairs added to the context.
func (a *SlogAdapter) With(args ...any) *SlogAdapter {
	return &SlogAdapter{logger: a.logger.With(args...)}
}

// WithGroup returns a new SlogAdapter with the given group name.
func (a *SlogAdapter) WithGroup(name string) *SlogAdapter {
	return &SlogAdapter{logger: a.logger.WithGroup(name)}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run TestSlogAdapter -count=1 -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /tmp/gca-modular
git add slog_adapter.go slog_adapter_test.go
git commit -m "feat: add SlogAdapter wrapping *slog.Logger for Logger interface"
```

---

### Task 12: Module Metrics Hooks

**Files:**
- Create: `metrics.go` — MetricsProvider interface, ModuleMetrics type, CollectAllMetrics
- Modify: `application.go` — add `CollectAllMetrics` method
- Create: `metrics_test.go` — tests

**Step 1: Write the failing test**

Create `metrics_test.go`:

```go
package modular

import (
	"context"
	"testing"
)

type metricsModule struct {
	name string
}

func (m *metricsModule) Name() string              { return m.name }
func (m *metricsModule) Init(app Application) error { return nil }
func (m *metricsModule) CollectMetrics(ctx context.Context) ModuleMetrics {
	return ModuleMetrics{
		Name: m.name,
		Values: map[string]float64{
			"requests_total": 100,
			"error_rate":     0.02,
		},
	}
}

func TestCollectAllMetrics(t *testing.T) {
	modA := &metricsModule{name: "api"}
	modB := &pluginModule{name: "no-metrics"} // doesn't implement MetricsProvider

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(modA, modB),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	stdApp := app.(*StdApplication)
	metrics := stdApp.CollectAllMetrics(context.Background())

	if len(metrics) != 1 {
		t.Fatalf("expected 1 metrics result, got %d", len(metrics))
	}
	if metrics[0].Name != "api" {
		t.Errorf("expected api, got %s", metrics[0].Name)
	}
	if metrics[0].Values["requests_total"] != 100 {
		t.Errorf("expected requests_total=100, got %v", metrics[0].Values["requests_total"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /tmp/gca-modular && go test -run TestCollectAllMetrics -count=1 -v`
Expected: FAIL — `MetricsProvider` undefined

**Step 3: Implement**

Create `metrics.go`:

```go
package modular

import "context"

// ModuleMetrics holds metrics collected from a single module.
type ModuleMetrics struct {
	Name   string
	Values map[string]float64
}

// MetricsProvider is an optional interface for modules that expose operational metrics.
// The framework collects metrics from all implementing modules on demand.
type MetricsProvider interface {
	CollectMetrics(ctx context.Context) ModuleMetrics
}
```

Add method to `application.go`:

```go
// CollectAllMetrics gathers metrics from all modules implementing MetricsProvider.
func (app *StdApplication) CollectAllMetrics(ctx context.Context) []ModuleMetrics {
	var results []ModuleMetrics
	for _, module := range app.moduleRegistry {
		if mp, ok := module.(MetricsProvider); ok {
			results = append(results, mp.CollectMetrics(ctx))
		}
	}
	return results
}
```

**Step 4: Run test to verify it passes**

Run: `cd /tmp/gca-modular && go test -run TestCollectAllMetrics -count=1 -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd /tmp/gca-modular && go test ./... -count=1`
Expected: All tests pass

**Step 6: Commit**

```bash
cd /tmp/gca-modular
git add metrics.go metrics_test.go application.go
git commit -m "feat: add MetricsProvider interface and CollectAllMetrics"
```

---

## Post-Implementation

After all 12 tasks are complete:

1. Run full test suite: `cd /tmp/gca-modular && go test ./... -count=1 -race`
2. Run linter: `cd /tmp/gca-modular && golangci-lint run`
3. Run vet: `cd /tmp/gca-modular && go vet ./...`
4. Fix any issues found

All work is on the `feat/reimplementation` branch. Create a PR against `main` when complete.
