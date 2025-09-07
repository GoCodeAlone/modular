# Contract: Dynamic Configuration Reload

## Purpose
Permit safe, selective in-process application of configuration changes without full restart.

## Interfaces (Conceptual Go)
```go
type ConfigDiff struct {
    Changed map[string]FieldChange
    Timestamp time.Time
}

type FieldChange struct {
    Old any
    New any
}

type Reloadable interface {
    Reload(ctx context.Context, diff ConfigDiff) error
}
```

## Lifecycle
1. Trigger (manual API or file watcher integration)
2. Collect updated configuration via feeders
3. Validate full configuration
4. Derive diff for dynamic-tagged fields
5. If diff empty: emit ConfigReloadNoop
6. Sequentially invoke Reload(ctx,diff) for subscribed modules (original start order)
7. Emit ConfigReloadCompleted (success/failure)

## Events
- ConfigReloadStarted { changed_count, timestamp }
- ConfigReloadNoop { timestamp }
- ConfigReloadCompleted { changed_count, duration_ms, success, error? }

## Constraints
- Reload MUST be idempotent
- Long-running operations inside Reload discouraged (<50ms typical)
- Errors abort remaining reload sequence, emit failure event
