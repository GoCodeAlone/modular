# Contract: Lifecycle Events (Conceptual)

## Purpose
Emit structured events for module lifecycle transitions consumable by observers and external systems.

## Events
- ModuleRegistering
- ModuleStarting
- ModuleStarted
- ModuleStopping
- ModuleStopped
- ModuleError

## Payload Fields (Core)
- timestamp
- moduleName
- phase
- details (map)
- correlationID (optional)

## Observer Semantics
- Non-blocking delivery; slow observer handling: buffered with backpressure warning event
- Failure in observer: logged + does not abort lifecycle (unless configured strict)
