# Contract: Scheduler (Conceptual)

## Purpose
Define scheduling of recurring jobs with bounded catch-up policy.

## Job Definition
- id
- cronExpression
- maxConcurrency
- catchUpPolicy (skip|bounded)
- backfillLimit (count or duration window)

## Operations
- Register(jobDef, handler) → error
- Start() → error
- Stop() → error
- ListJobs() → []JobDef

## Guarantees
- No overlapping executions when maxConcurrency=1
- Backfill respects policy constraints

## Error Cases
- ErrInvalidCron
- ErrDuplicateJob
