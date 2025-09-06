# Contract: Configuration System (Conceptual)

## Purpose
Merge multi-source configuration with validation, defaults, provenance, and dynamic reload support.

## Operations
- Load(feederSet) → ConfigTree|error
- Validate(config) → []ValidationError
- ApplyDefaults(config) → Config
- GetProvenance(fieldPath) → ProvenanceRecord
- Reload(dynamicFieldsDelta) → []ReloadResult

## Constraints
- Required fields enforced pre-start
- Dynamic-only reload safety
- Provenance redacts secret values

## Error Cases
- ErrMissingRequired(field)
- ErrInvalidValue(field, reason)
