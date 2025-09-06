# Data Model (Conceptual)

## Entities

### Application
Purpose: Orchestrates module lifecycle, configuration aggregation, service registry access.
Key State:
- RegisteredModules[]
- ServiceRegistry (map[name|interface]→Provider)
- TenantContexts (map[tenantID]→TenantContext)
- InstanceContexts (map[instanceID]→InstanceContext)
- Observers[]

### Module
Attributes:
- Name
- Version
- DeclaredDependencies[] (name/interface, optional flag)
- ProvidesServices[] (name/interface, scope: global|tenant|instance)
- ConfigSpec (schema metadata)
- DynamicFields[] (subset of config keys)

### Configuration Object
Fields:
- FieldName
- Type
- DefaultValue (optional)
- Required (bool)
- Description
- Dynamic (bool)
- Provenance (feeder ID)
Validation Rules:
- Must satisfy type
- Required fields set post-merge
- Custom validator returns nil/error

### TenantContext
Fields:
- TenantID
- TenantConfig (merged tenant-specific config)
- CreatedAt

### InstanceContext
Fields:
- InstanceID
- InstanceConfig (merged instance-specific config)

### Service Registry Entry
Fields:
- Key (name or interface signature)
- ProviderModule
- Scope (global|tenant|instance)
- Priority (int)
- RegistrationTime

### Lifecycle Event
Fields:
- Timestamp
- ModuleName
- Phase (registering|starting|started|stopping|stopped|error)
- Details (string / structured map)

### Health Status
Fields:
- ModuleName
- Status (healthy|degraded|unhealthy)
- Message
- LastUpdated

### Scheduled Job Definition
Fields:
- JobID
- CronExpression
- MaxConcurrency
- CatchUpPolicy (skip|boundedBackfill)
- BackfillLimit (executions or duration)

### Event Message
Fields:
- Topic
- Headers (map)
- Payload (abstract, validated externally)
- CorrelationID

### Certificate Asset
Fields:
- Domains[]
- Expiry
- LastRenewalAttempt
- Status (valid|renewing|error)

## Relationships
- Application 1..* Module
- Module 0..* Service Registry Entry
- Application 0..* TenantContext
- Application 0..* InstanceContext
- Module 0..* Lifecycle Event
- Module 0..* Health Status (latest over time)
- Scheduler 0..* Scheduled Job Definition
- EventBus 0..* Event Message

## State Transitions (Module Lifecycle)
```
registered -> starting -> started -> stopping -> stopped
                        -> error (terminal for failed start)
```
Rules:
- Cannot transition from stopped to started without full re-registration cycle.
- Error during starting triggers rollback (stop previously started modules).

## Validation Summary
- Configuration: Required + custom validator pass before Start invoked.
- Dynamic reload: Only fields flagged dynamic may change post-start; triggers re-validation.
- Service registration: Duplicate (same key + scope) rejected unless explicit override policy defined.

## Open Extension Points
- Additional error categories
- Additional service scopes (e.g., request) future
- Additional auth mechanisms (SAML, mTLS) future
