# Contract: Health & Readiness (Conceptual)

## Purpose
Provide aggregate and per-module health for orchestration and automation.

## Module Report
- status: healthy|degraded|unhealthy
- message
- timestamp

## Aggregation Rules
- Readiness excludes optional module failures
- Health = worst(status) across required modules

## Operations
- Report(moduleStatus) → error
- GetModuleStatus(name) → Status|error
- GetAggregateHealth() → AggregateStatus
- SubscribeChanges(callback)
