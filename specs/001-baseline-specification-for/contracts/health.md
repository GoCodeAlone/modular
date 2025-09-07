# Contract: Aggregate Health API (Conceptual)

## Purpose
Provide consistent retrieval of current health & readiness snapshot for automation and monitoring.

## Interface (Conceptual Go)
```go
type AggregateHealthService interface {
    Snapshot() AggregateHealthSnapshot
}
```

HTTP Endpoint (optional future): `GET /healthz`
- 200 OK: readiness healthy or degraded (JSON snapshot)
- 503 Service Unavailable: readiness unhealthy

## JSON Schema (Snapshot)
```json
{
  "type": "object",
  "properties": {
    "generated_at": {"type": "string", "format": "date-time"},
    "overall_status": {"type": "string", "enum": ["healthy","degraded","unhealthy"]},
    "readiness_status": {"type": "string", "enum": ["healthy","degraded","unhealthy"]},
    "modules": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "status": {"type": "string", "enum": ["healthy","degraded","unhealthy"]},
          "message": {"type": "string"},
          "optional": {"type": "boolean"},
          "timestamp": {"type": "string", "format": "date-time"}
        },
        "required": ["name","status","timestamp"],
        "additionalProperties": false
      }
    }
  },
  "required": ["generated_at","overall_status","readiness_status","modules"],
  "additionalProperties": false
}
```

## Events
- HealthEvaluated: emitted after every aggregation cycle with snapshot hash & counts.
