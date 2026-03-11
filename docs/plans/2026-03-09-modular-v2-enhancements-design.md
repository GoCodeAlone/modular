# Modular v2 Enhancements Design

**Goal:** Address 12 gaps identified in the Modular framework audit, making it a more complete foundation for the Workflow engine and other consumers.

**Delivery:** Single PR on the `feat/reimplementation` branch.

**Consumer:** GoCodeAlone/workflow engine (primary), other Go services using Modular.

---

## Section 1: Core Lifecycle

### 1.1 Config-Driven Dependency Hints

**Gap:** Modules can declare dependencies via `DependencyAware` interface, but there's no way to declare them from the builder/config level without modifying module code.

**Design:** `WithModuleDependency(from, to string)` builder option injects edges into the dependency graph before resolution. These hints feed into the existing topological sort alongside `DependencyAware` edges.

```go
app := modular.NewApplicationBuilder().
    WithModuleDependency("api-server", "database").
    WithModuleDependency("api-server", "cache").
    Build()
```

Implementation: Store hints in `[]DependencyEdge` on the builder, merge into the graph in `resolveDependencies()` before DFS.

### 1.2 Drainable Interface (Shutdown Drain Phases)

**Gap:** `Stoppable` has a single `Stop()` method. No way to drain in-flight work before hard stop.

**Design:** New `Drainable` interface with `PreStop(ctx)` called before `Stop()`:

```go
type Drainable interface {
    PreStop(ctx context.Context) error
}
```

Shutdown sequence: `PreStop` all drainable modules (reverse dependency order) → `Stop` all stoppable modules (reverse dependency order). `PreStop` context has a configurable timeout via `WithDrainTimeout(d)`.

### 1.3 Application Phase Tracking

**Gap:** No way to query what lifecycle phase the application is in.

**Design:** `Phase()` method on Application returning an enum:

```go
type AppPhase int
const (
    PhaseCreated AppPhase = iota
    PhaseInitializing
    PhaseStarting
    PhaseRunning
    PhaseDraining
    PhaseStopping
    PhaseStopped
)
```

Phase transitions emit CloudEvents (`EventTypeAppPhaseChanged`) if a Subject is configured.

### 1.4 Parallel Init at Same Topological Depth

**Gap:** Modules at the same depth in the dependency graph are initialized sequentially.

**Design:** `WithParallelInit()` builder option. When enabled, modules at the same topological depth are initialized concurrently via `errgroup`. Modules at different depths remain sequential (respecting dependency order).

Disabled by default for backward compatibility. Errors from any goroutine cancel the group and return the first error.

---

## Section 2: Services & Plugins

### 2.1 Type-Safe Service Helpers

**Gap:** `RegisterService`/`GetService` use `interface{}`, requiring type assertions at every call site.

**Design:** Package-level generic helper functions (not methods, since Go interfaces can't have type parameters):

```go
func RegisterTypedService[T any](registry ServiceRegistry, name string, svc T) error
func GetTypedService[T any](registry ServiceRegistry, name string) (T, error)
```

These wrap the existing `RegisterService`/`GetService` with compile-time type safety. `GetTypedService` returns a typed zero value + error on type mismatch.

### 2.2 Service Readiness Events

**Gap:** No notification when a service becomes available, making lazy/async resolution brittle.

**Design:** `EventTypeServiceRegistered` CloudEvent emitted by `EnhancedServiceRegistry.RegisterService()`. Plus `OnServiceReady(name, callback)` method that fires the callback immediately if already registered, or defers until registration.

```go
registry.OnServiceReady("database", func(svc interface{}) {
    db := svc.(*sql.DB)
    // use db
})
```

### 2.3 Plugin Interface

**Gap:** No standard way to bundle modules, services, and hooks as a distributable unit.

**Design:** Three interfaces with progressive capability:

```go
type Plugin interface {
    Name() string
    Modules() []Module
}

type PluginWithHooks interface {
    Plugin
    InitHooks() []func(Application) error
}

type PluginWithServices interface {
    Plugin
    Services() []ServiceDefinition
}

type ServiceDefinition struct {
    Name    string
    Service interface{}
}
```

Builder gains `WithPlugins(...Plugin)`: registers all modules, runs hooks during init, registers services before module init.

---

## Section 3: Configuration & Reload

### 3.1 ReloadOrchestrator Integration

**Gap:** `ReloadOrchestrator` exists but isn't wired into the Application lifecycle.

**Design:** `WithDynamicReload()` builder option:
- Creates `ReloadOrchestrator` during `Build()`
- Auto-registers all `Reloadable` modules after init
- Calls `Start()` during app start, `Stop()` during app stop
- Exposes `Application.RequestReload(ctx, trigger, diff)` for consumers

### 3.2 Config File Watcher

**Gap:** No built-in file watching for configuration changes.

**Design:** New `modules/configwatcher` package providing a module that watches config files:

```go
watcher := configwatcher.New(
    configwatcher.WithPaths("config/app.yaml", "config/overrides.yaml"),
    configwatcher.WithDebounce(500 * time.Millisecond),
    configwatcher.WithDiffFunc(myDiffFunc),
)
```

Uses `fsnotify` (single new dependency). On change: debounce → compute diff → call `Application.RequestReload()`. Implements `Startable`/`Stoppable` for lifecycle management.

### 3.3 Secret Resolution Hooks

**Gap:** Config values like `${vault:secret/db-password}` have no standard expansion mechanism.

**Design:** `SecretResolver` interface + utility function:

```go
type SecretResolver interface {
    ResolveSecret(ctx context.Context, ref string) (string, error)
    CanResolve(ref string) bool
}

func ExpandSecrets(ctx context.Context, config map[string]any, resolvers ...SecretResolver) error
```

`ExpandSecrets` walks the config map, finds string values matching `${prefix:path}`, dispatches to the first resolver where `CanResolve` returns true, and replaces in-place. Called by consumers before feeding config to modules.

---

## Section 4: Observability

### 4.1 Slog Adapter

**Gap:** Framework uses custom `Logger` interface. Go's `slog` is the standard.

**Design:** Keep `Logger` interface unchanged. Add `SlogAdapter` implementing `Logger` by wrapping `*slog.Logger`:

```go
type SlogAdapter struct {
    logger *slog.Logger
}

func NewSlogAdapter(l *slog.Logger) *SlogAdapter
func (a *SlogAdapter) With(args ...any) *SlogAdapter
func (a *SlogAdapter) WithGroup(name string) *SlogAdapter
```

`With()`/`WithGroup()` return `*SlogAdapter` (not `Logger`) for chaining structured context. Base `Logger` interface methods (`Info`, `Error`, `Warn`, `Debug`) delegate to slog equivalents.

### 4.2 Module Metrics Hooks

**Gap:** No standard way for modules to expose operational metrics.

**Design:** Optional `MetricsProvider` interface:

```go
type ModuleMetrics struct {
    Name   string
    Values map[string]float64
}

type MetricsProvider interface {
    CollectMetrics(ctx context.Context) ModuleMetrics
}
```

`Application.CollectAllMetrics(ctx) []ModuleMetrics` iterates modules implementing `MetricsProvider`. No OTEL/Prometheus dependency — returns raw values for consumers to map to their telemetry system.

---

## Gap Matrix Summary

| # | Gap | Section | Key Types |
|---|-----|---------|-----------|
| 1 | Config-driven dependency hints | 1.1 | `WithModuleDependency` |
| 2 | Shutdown drain phases | 1.2 | `Drainable`, `PreStop` |
| 3 | Application phase tracking | 1.3 | `AppPhase`, `Phase()` |
| 4 | Parallel init | 1.4 | `WithParallelInit` |
| 5 | Type-safe services | 2.1 | `RegisterTypedService[T]` |
| 6 | Service readiness events | 2.2 | `OnServiceReady` |
| 7 | Plugin interface | 2.3 | `Plugin`, `WithPlugins` |
| 8 | Reload orchestrator integration | 3.1 | `WithDynamicReload` |
| 9 | Config file watcher | 3.2 | `configwatcher` module |
| 10 | Secret resolution hooks | 3.3 | `SecretResolver` |
| 11 | Slog adapter | 4.1 | `SlogAdapter` |
| 12 | Module metrics hooks | 4.2 | `MetricsProvider` |
