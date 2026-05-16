# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**Priority order when making decisions:** Architectural constraints → Correctness → Scope control → Verification → Speed.

---

## 1. Repository Architecture

### 1.1. Binaries & Structure

Three independent binaries share one codebase, all wired via `internal/bootstrap/` using Google Wire:
- `cmd/api` — HTTP REST API (Gin)
- `cmd/worker` — Async event processor (Kafka consumer)
- `cmd/scheduler` — Cron jobs

### 1.2. Strict Module Layout (Clean Architecture)

Each business domain lives under `internal/modules/<name>/` with a strict four-layer layout. **Dependency direction is always inward: `http → app → domain ← infra`.**

```
domain/   ← Pure business logic: entities, value objects, domain errors, repo interfaces. Zero external imports.
app/      ← Application layer: commands, queries, DTOs, service, ports (interfaces for infra dependencies).
infra/    ← Adapters implementing ports: postgres repos, external service clients.
http/     ← Gin handlers, request/response types, middleware, route registration.
```

**Interface placement rule:**
- Interfaces that `app/service/` depends on (Redis, email, auth, clock, event publisher) → `app/ports/`
- Interfaces that `http/` depends on (use-cases consumed by handlers) → defined locally in the handler file that uses them (`type XUseCase interface { ... }`)
- Never define port interfaces in `infra/` or `domain/`

### 1.3. Postgres Repository File Layout

Every entity's repository in `infra/postgres/` is split across these files:

| File | Responsibility |
|---|---|
| `rows.go` | DB row structs — mirror of DB columns, NOT domain entities |
| `scan.go` | `scanX(row pgx.Row)` — one scan function per entity type |
| `mapper.go` | `mapXRowToEntity()` / `mapXRowToView()` — row → domain type |
| `query_repo.go` | Read-only queries (SELECT only) |
| `<entity>_repo.go` | Write queries (INSERT / UPDATE / DELETE) for that entity |

SQL constants naming convention: `queryGetX`, `queryListX`, `queryInsertX`, `queryUpdateX`.

### 1.4. Dependency Injection (Wire)

`internal/bootstrap/` is the composition root:
- `wire.go` — Build-time injector (`//go:build wireinject`). Edit here to add providers.
- `wire_gen.go` — **Auto-generated. Never edit directly.** Regenerate with `make wire`.
- `providers.go` — Provider functions grouped into `wire.NewSet()` sets: `PlatformSet`, `HTTPSet`, `IdentityModuleSet`, `APISet`.

**When adding a new injectable dependency:**
1. Write the provider function in the module's `infra/adapters/provider.go` or `http/providers.go`
2. Add it to that file's `ProviderSet = wire.NewSet(...)`
3. Run `make wire` — Wire will report missing providers or type conflicts
4. If Wire reports "no provider for type X", add `wire.Bind(new(Interface), new(ConcreteImpl))` in the relevant ProviderSet

### 1.5. Platform Packages (`internal/platform/`)

Cross-cutting infrastructure shared by all modules:

| Package | Responsibility |
|---|---|
| `config/` | `MustLoad()` reads all env vars; panics on missing required vars |
| `auth/` | JWT generation/validation, bcrypt, Redis verify-token service |
| `httpx/` | Gin engine factory (CORS, logging, recovery), `httpx.Success()` / `httpx.Error()` helpers |
| `tx/` | `tx.Manager` — context-based Postgres transaction propagation |
| `outbox/` | Transactional outbox: write events atomically with business TX, dispatch to Kafka separately |
| `idempotency/` | Request deduplication via `Idempotency-Key` header (Redis + DB backed) |
| `db/` | `pgxpool` setup with health checks |
| `logger/` | Zap logger, globally registered via `zap.ReplaceGlobals()` |

---

## 2. Coding Contracts

### 2.1. HTTP Response Contract

All handlers **must** use `httpx.Success` and `httpx.Error`. Never write raw `c.JSON`:

```go
httpx.Success(c, http.StatusOK, "MESSAGE_CODE", payload)
httpx.Error(c, http.StatusNotFound, "ERROR_CODE", "human message")
// Response shape: { "success": bool, "code": "...", "message": "...", "data": ... }
```

**Domain error → HTTP status mapping.** Each handler file defines its own `writeXError` helper using sentinel errors from `domain/error/`:

```go
func writeAddressError(c *gin.Context, op string, err error) {
    switch {
    case errors.Is(err, identity_err.ErrAddressNotFound):
        httpx.Error(c, http.StatusNotFound, "ADDRESS_NOT_FOUND", "address not found")
    case errors.Is(err, identity_err.ErrForbidden):
        httpx.Error(c, http.StatusForbidden, "FORBIDDEN", "access denied")
    default:
        httpx.Error(c, http.StatusInternalServerError, op+"_FAILED", "internal server error")
    }
}
```

Never use `err.Error()` string matching. Always use `errors.Is()`.

### 2.2. Handler Helpers

Two helpers are available in every module's `http/` package:

```go
// Binds JSON and writes validation error automatically. Return immediately on false.
if !bindJSON(c, &req) { return }

// Reads UserID from AuthContext. Writes 401 and returns false if missing.
userID, ok := getAuthUser(c)
if !ok { return }

// When you need the full AuthContext (JTI, SessionID, DeviceID):
ac, ok := middleware.GetAuthContext(c)
if !ok { return }
```

These helpers call `c.Abort()` internally — just `return` after a false result, never call `httpx.Error` again.

### 2.3. IDOR Check (Mandatory on All Mutations)

Any handler that mutates a resource identified by a URL param must verify ownership. **Never skip this.**

```go
// In service, after fetching entity by ID:
if err := entity.AssertOwnership(cmd.UserID); err != nil {
    return err  // ErrForbidden → 403 in writeXError
}
```

URL parameters are user-controlled. Never assume the authenticated user can only reach their own resources.

### 2.4. Transaction Pattern

```go
err = s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
    if err := s.repo.Update(txCtx, ...); err != nil { return err }
    return s.other.Insert(txCtx, ...)
})
// Post-transaction cache cleanup uses the original ctx, not txCtx:
s.redisService.Delete(ctx, key)
```

**Critical rules:**
- Never share `txCtx` across goroutines
- Never use the outer `ctx` inside the transaction function — always use `txCtx`
- Cache/Redis cleanup after the transaction always uses the original `ctx`

### 2.5. Event Publishing (Outbox Pattern)

Events must be recorded inside the business transaction to guarantee delivery:

```go
// Inside WithinTransaction — use txCtx:
s.eventPublisher.Publish(txCtx, event.UserRegistered{...})
// Writes to outbox_events atomically with business write.
// Dispatcher worker reads outbox_events and forwards to Kafka.
```

### 2.6. Database Queries (pgx v5)

Scan column order **must exactly match** the SELECT column list. If a query selects 13 columns, `row.Scan()` must have exactly 13 destinations in the same order. Mismatches cause **runtime panics, not compile errors**.

### 2.7. Redis Key Prefixes

Established prefixes — never reuse for other purposes:

| Key pattern | Purpose |
|---|---|
| `blacklist:access_token:{jti}` | Revoked access token JTIs |
| `identity:email_verify:{token}` | Email verification tokens |
| `refresh_token:{hash}` | Active session refresh tokens |

### 2.8. Migrations

Sequential numbering is mandatory (`000001_name.up.sql` / `.down.sql`). Each `.down.sql` must exactly undo its `.up.sql`. Create with `make migrate-create name=<description>`.

### 2.9. Middleware Ordering

- `AuthMiddleware` — validates Bearer JWT, stores `AuthContext` in gin context. Read with `middleware.GetAuthContext(c)`.
- `StrictAuthMiddleware` — checks `AuthContext.JTI` against Redis blacklist. **Must run after `AuthMiddleware`** — it reads from `AuthContext`.
- `idempotencyMiddleware` — deduplicates non-idempotent mutations (POST/DELETE).

---

## 3. Adding a New Feature (Touch Order)

Work domain-inward first to avoid forward-reference compile errors:

1. `domain/entity/` — add/update entity fields
2. `domain/repository.go` — add method signature to repo interface
3. `infra/postgres/rows.go` → `scan.go` → `mapper.go` → `<entity>_repo.go` — implement DB layer
4. `app/command/` or `app/query/` — define command/query struct
5. `app/service/` — implement business logic
6. `http/request.go` — add request struct with binding tags
7. `http/response.go` — add response struct and message constants
8. `http/<entity>_handler.go` — add handler method
9. `http/router.go` — register route

If adding an external dependency: add interface to `app/ports/` → implement adapter in `infra/adapters/` → add to `ProviderSet` → run `make wire`.

---

## 4. Commands

```bash
make tools                                       # Install wire, air, golangci-lint, migrate
make dev / make dev-worker / make dev-scheduler  # Hot reload development
make build / make build-api                      # Build binaries
make wire                                        # Regenerate DI after provider changes
make migrate-up / migrate-down
make migrate-create name=<description>           # Creates 00000N_name.up/down.sql
make infra-up / infra-down / infra-reset         # Docker Compose (Postgres, Redis, Kafka)
make test                                        # With race detector
make test-coverage
make lint / make fmt / make vet / make tidy

# Run a single test:
go test ./internal/modules/identity/... -run TestFunctionName -v
```

---

## 5. Behavioral Guidelines

### 5.1. Think Before Coding

- Do not assume. If important context is missing, stop and ask explicitly.
- If multiple interpretations exist, present them — don't silently pick one.
- If a simpler approach exists, say so and push back.

### 5.2. Simplicity First

- Implement the minimum code that solves the stated problem. Nothing speculative.
- No extra features, no premature abstractions, no configurability unless requested.

### 5.3. Surgical Changes Only

- Change only files directly related to the request.
- Match existing style and patterns exactly, even when you'd do it differently.
- Remove only imports/variables/functions that YOUR changes made unused.
- Do not perform drive-by cleanups on adjacent code, comments, or formatting.

### 5.4. Scope Control

- Prefer backward-compatible changes.
- Prefer existing project patterns over idealized architecture.
- Replacing established patterns or cross-module refactoring requires explicit user confirmation.

---

## 6. Execution & Verification Contract

### 6.1. Never Present Unverified Code as Complete

Run verification in this order:
1. `go build ./...` — catches compile errors and type mismatches
2. `go test ./...` or targeted: `go test ./internal/modules/<name>/... -run TestX -v`
3. `make lint` — for lint regressions

If verification fails, explain why and fix deliberately. Do not loop blindly.

### 6.2. Response Format for Non-Trivial Tasks

1. **Understanding** — restate the task and identify root cause / affected files
2. **Assumptions** — list missing context or constraints
3. **Plan** — concise execution steps
4. **Implementation** — make only the requested changes
5. **Verification** — state what was run and the result
6. **Summary** — list changed files

### 6.3. Task Playbooks

**Bug Fix:** Reproduce → identify root cause → minimal fix → verify with `go build` + targeted test. Do not refactor unrelated code.

**New Feature:** Confirm integration points → follow touch order in §3 → implement only what was requested → verify build + lint.

**Refactor:** Preserve behavior exactly → run tests before and after → keep diff readable.

**Code Review:** Focus on correctness, ownership (IDOR) checks, pgx scan-column order, transaction context discipline, and missing domain error mappings.
