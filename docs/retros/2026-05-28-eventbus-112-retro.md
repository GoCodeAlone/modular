# Retro: eventbus #112 — timer-drain hang + silent-drop observability

PR #116 · squash `ca4bf71` · merged 2026-05-29 · closes #112

## What shipped

Three bugs fixed in `modules/eventbus` in one PR:

1. **P1 timer-drain hang** (`memory.go`) — `deliveryMode: "timeout"` Publish goroutines hung forever on Go 1.23+ because the legacy `if !Stop() { <-C }` drain re-read an already-drained timer channel. Fixed with a non-blocking select drain.
2. **P3 silent-drop observability** (`memory.go`, `custom_memory.go`) — events abandoned in `sub.eventCh` on `Unsubscribe`/`Stop` were uncounted; async `workerPool` residual tasks also uncounted. Added `drainSubscription` + `drainWorkerPool` called from the handler goroutine after every exit, counting each abandoned event as dropped.
3. **P2 `CustomMemoryEventBus` stats gap** (`engine_registry.go`) — `CollectStats`/`CollectPerEngineStats` type-asserted `*MemoryEventBus`, excluding the custom engine. Replaced with a `statsProvider` interface so both engines participate.

Supporting: `CustomMemoryEventBus` gained `sync.WaitGroup` + `finished` channel so `Stop` waits for handler goroutines and `Stats()` is final on return. False "all in-flight events processed" doc comments corrected in `eventbus.go` and `module.go`.

New invariant (gated by tests): once publishers are quiesced, `delivered + dropped == enqueued` for both sync and async subscriptions across both engines.

## Gates — what worked

### Adversarial design review — CAUGHT 2 Criticals + 4 Importants pre-code

The design review (Rev 1 → Rev 2) caught the two most expensive bugs before any code was written:

- **C1 (async conservation gap)** — the draft plan drained `sub.eventCh` in `handleEvents` but omitted `workerPool` residual tasks; conservation would have been false for async subscriptions. Fixed in D4 (`drainWorkerPool` after `wg.Wait`).
- **C2 (count partition ambiguity)** — the draft did not specify where each event was counted; double-counting between `drainSubscription` and `drainWorkerPool` was possible. Fixed by the exact-count-partition table in D2, which assigns each terminal transition to exactly one site.
- **I1 (CI-safe hang test)** — original T1 would have used a naked `Publish` call that deadlocks CI on unpatched code. Revised to a goroutine + `select` with 3s assertion.
- **I3 (CustomMemoryEventBus shutdown)** — without the `wg`/`finished` infra added in D5, a post-`Stop` `Stats()` read would have raced the drain goroutine; the adversarial pass flagged the missing synchronization.

### TDD (RED→GREEN) — tests written first, failed on unpatched code, passed on patched

`issue112_memory_test.go` and `issue112_custom_test.go` were written before implementation changes. The hang test and all conservation tests verified RED on unpatched code (P1 visible via goroutine timeout, P3 via `delivered+dropped != enqueued` assertions) and GREEN after the fixes.

### Code review — PASS (no blocking findings)

CI ran `Test eventbus` with `-race`, `Lint eventbus`, `Verify eventbus`, full BDD matrix, CodeQL — all green. No race conditions. 25 checks passed; 0 failed.

## What to carry forward

- **Non-blocking timer drain pattern** — the `select { case <-C: default: }` idiom should be the house standard for all timer drains in this repo (replace any remaining `if !Stop() { <-C }`).
- **`DurableMemoryEventBus.Stats()` arity mismatch** — returns `(delivered uint64)` (single return), so it still does not satisfy `statsProvider` and is skipped by `CollectStats`. Out of scope for #112; file a follow-up issue.
- **Multi-engine config threading** — `deliveryMode`/`publishBlockTimeout` are not threaded into per-engine `Config` maps on the multi-engine path. Pre-existing; timer fix still applies in single-engine timeout mode. Follow-up.
- **Residual publish-after-drain race** — documented and accepted in ADR 0001. Closing it fully requires holding the subscription mutex across the channel send (serializes all publishes). Not worth the cost for an observability fix; revisit if contention becomes a production concern.
