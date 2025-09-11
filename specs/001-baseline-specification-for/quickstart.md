# Quickstart: Enabling Dynamic Reload & Health Aggregation

## 1. Enable Features
```go
app := modular.New(
    modular.WithDynamicReload(),
    modular.WithHealthAggregator(),
    modular.WithTenantGuardMode(modular.TenantGuardStrict),
)
```

## 2. Tag Dynamic Config Fields
```go
type HTTPConfig struct {
    Port int    `yaml:"port" default:"8080" desc:"HTTP listen port"`
    ReadTimeout time.Duration `yaml:"read_timeout" default:"5s" desc:"Server read timeout" dynamic:"true"`
}
```

## 3. Implement Reloadable (Module)
```go
func (m *HTTPServerModule) Reload(ctx context.Context, diff modular.ConfigDiff) error {
    if diff.Has("read_timeout") {
        m.server.SetReadTimeout(diff.Duration("read_timeout"))
    }
    return nil
}
```

## 4. Expose Health
```go
func (m *HTTPServerModule) HealthReport(ctx context.Context) modular.HealthResult {
    return modular.HealthResult{Status: modular.Healthy, Message: "ok", Timestamp: time.Now()}
}
```

## 5. Query Aggregate Health
```go
agg := app.Health()
snap := agg.Snapshot()
fmt.Println("overall:", snap.OverallStatus, "readiness:", snap.ReadinessStatus)
```

## 6. Trigger Reload (Example)
```go
// After updating configuration sources externally
if err := app.Reload(ctx); err != nil { log.Fatal(err) }
```

## 7. Observe Events
- ConfigReloadStarted / ConfigReloadCompleted
- HealthEvaluated (snapshot)

## 8. Next Steps
- Add scheduler catch-up policy: WithSchedulerCatchUp(modular.CatchUpPolicyBounded{MaxExecutions:10, MaxWindow: time.Hour})
- Register OIDC provider(s): auth.WithOIDCProvider(myProvider)
- Implement secret wrapping for sensitive config values.
