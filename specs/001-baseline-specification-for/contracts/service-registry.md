# Contract: Service Registry (Conceptual)

## Purpose
Provide lookup and registration for services by name or interface with deterministic ambiguity resolution.

## Operations
- Register(serviceDescriptor) → error
- ResolveByName(name) → Service|error
- ResolveByInterface(interfaceType) → Service|error (apply tie-break order)
- ListServices(scope?) → []ServiceDescriptor

## Constraints
- O(1) expected lookup
- Ambiguity: apply tie-break (explicit name > priority > registration time)
- Tenant / instance scope isolation enforced

## Error Cases
- ErrNotFound
- ErrAmbiguous (includes candidates)
- ErrDuplicateRegistration
