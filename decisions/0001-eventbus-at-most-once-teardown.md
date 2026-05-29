# 1. Eventbus in-memory engines: at-most-once delivery across teardown

Date: 2026-05-28
Status: Accepted
Context: issue #112

## Context

The in-memory event-bus engines (`MemoryEventBus`, `CustomMemoryEventBus`)
buffer events in per-subscriber channels and, for async memory subscriptions, in
a shared worker pool. On `Unsubscribe` and `Stop` the handler goroutines exited
immediately and **abandoned** any events still buffered: they were neither
delivered nor counted as dropped. `Stats()` reported zero drops while events
silently evaporated — invisible to tests and to anyone wiring `droppedCount` to
alerting (issue #112, part 3). The `EventBus.Stop` doc comment additionally
claimed Stop "ensure[s] all in-flight events are processed before returning,"
which the implementation never honored.

A contract had to be chosen and documented. Two options:

1. **Best-effort deliver at teardown** — drain buffered events through the
   handler before exiting.
2. **At-most-once; count abandoned events as dropped** — do not deliver buffered
   events at teardown; increment `droppedCount` for each so the loss is
   observable and `Stats()` conservation (`delivered + dropped == enqueued`)
   holds.

## Decision

Adopt **at-most-once delivery across teardown** (option 2).

- On `Unsubscribe`/`Stop`, events still buffered in a subscriber channel are
  counted as dropped, not delivered (`drainSubscription`).
- For async `MemoryEventBus` subscriptions, events dequeued into the worker pool
  but not executed before `Stop` are also counted as dropped, drained after
  `wg.Wait()` so the operation is race-free (`drainWorkerPool`).
- `CustomMemoryEventBus` gains the `sync.WaitGroup` + `finished`-channel
  shutdown synchronization the standard engine already had, so `Stop` waits for
  handler goroutines to finish draining and `Stats()` is final on return.
- The misleading `Stop` doc comments in `eventbus.go` and `module.go` are
  corrected to state the at-most-once contract.

## Rejected: best-effort deliver at teardown

`Stop` cancels the bus context first, so any handler invoked during drain would
run with an already-cancelled context. A slow or blocking handler would stall
`Stop` (which has its own shutdown-deadline budget) and `Unsubscribe`. Delivering
on a dying bus is a footgun and contradicts the existing cancel-first ordering.
Counting-as-dropped is safe, observable, order-preserving, and makes a
conservation invariant testable. The goal of #112 is to make drops *visible*,
not to add best-effort delivery semantics the engines never promised.

## Consequences

- New invariant, gated by tests: once publishers are quiesced,
  `delivered + dropped == enqueued` for both sync and async subscriptions.
- **Residual race (accepted):** a publisher that passed the `cancelled`/
  `isStarted` check but has not yet sent can enqueue one event after the drain;
  it is then lost and uncounted. Fully closing this would require holding the
  subscription mutex across the channel send, serializing all publishes —
  rejected as too costly for an observability fix. Conservation holds exactly
  once publishers are quiesced (the realistic teardown path).
- **Out of scope (follow-ups):** `DurableMemoryEventBus.Stats()` has a single
  `(delivered)` return and so does not satisfy the new `statsProvider` interface
  (it never drops, by design); the multi-engine config path does not thread
  `deliveryMode`/`publishBlockTimeout` into per-engine config maps.
