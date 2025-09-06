# Quickstart – Modular Framework Baseline

## Goal
Stand up a modular application with HTTP server, auth, cache, and database modules using configuration layering.

## Steps
1. Define configuration files (base.yaml, instance.yaml, tenants/tenantA.yaml).
2. Export required secrets as environment variables (e.g., AUTH_JWT_SIGNING_KEY, DATABASE_URL).
3. Initialize application builder; register modules (order not required; framework sorts).
4. Provide feeders: env feeder > file feeder(s) > programmatic overrides.
5. Start application; verify lifecycle events and health endpoint.
6. Trigger graceful shutdown (SIGINT) and confirm reverse-order stop.

## Verification Checklist
- All modules report healthy.
- Auth validates JWT and rejects tampered token.
- Cache set/get round-trip works.
- Database connectivity established (simple query succeeds).
- Configuration provenance lists correct sources for sampled fields.
- Hot-reload a dynamic field (e.g., log level) and observe Reloadable invocation.

## Next Steps
- Add scheduler job and verify bounded backfill policy.
- Integrate event bus for async processing.
