# notiq — Remaining Fixes (Audit)

> Re-audit performed 2026-06-21. The recent commits already fixed the worker-pool,
> backoff-overflow, and webhook-delivery issues. The items below are what remains.
> Grouped by severity. Check off as completed.

---

## 🔴 High — correctness / security

- [x] **1. SSRF hole in the user-facing webhook job handler** — FIXED 2026-06-21
  `internal/worker/handlers/webhook.go:32` uses a plain `http.Client`, while the
  *delivery* handler was hardened with `safehttp.NewClient(...)`
  (`internal/worker/handlers/webhook_delivery.go:33`). A user can submit
  `{"type":"webhook","payload":{"url":"http://169.254.169.254/..."}}` and hit
  cloud metadata / internal services. The SSRF guard only protects
  registered-webhook delivery, not the `webhook` job type.
  → Route this handler through `safehttp` too (and respect `WEBHOOK_ALLOW_PRIVATE`).

- [x] **2. `webhook` usecase `Delete` masks all errors as 404** — FIXED 2026-06-21
  `internal/usecase/webhook/webhook.go:49-54` — any repo error (DB outage, etc.)
  is collapsed into `ErrWebhookNotFound`. The repo already distinguishes
  "not found" (rows==0) from real errors, so just return `err` unchanged.

- [x] **3. Orphaned pending jobs when enqueue fails** — FIXED 2026-06-21
  `internal/usecase/job/job.go:65-88` — the DB row is `Create`d, then if
  `queueClient.Enqueue` fails the function returns an error but the row stays
  `pending` forever (never in Redis, never processed). Needs compensation:
  delete/mark-failed the row, enqueue-then-persist, or an outbox.

- [x] **4. Negative `max_retries` not validated** — FIXED 2026-06-21
  `internal/delivery/http/handler/job.go:72` + `internal/usecase/job/job.go:49`
  only default when `== 0`. A `max_retries: -1` flows into `asynq.MaxRetry(-1)`
  and makes `IsLastAttempt` (`internal/worker/handlers/base.go:132`) true on the
  first failure → job goes straight to `dead`. Reject negatives in the
  `enqueueRequest` binding.

---

## 🟡 Medium — robustness / security hardening

- [x] **5. Admin basic-auth uses non-constant-time comparison** — FIXED 2026-06-21
  `internal/delivery/http/middleware/auth.go:16` — `u != username || p != password`
  is timing-attackable. Use `crypto/subtle.ConstantTimeCompare` / `hmac.Equal`.

- [x] **6. No validation that `ADMIN_PASSWORD` is set** — FIXED 2026-06-21
  `config/config.go` reads it but never enforces it. An empty configured password
  is a weak admin surface. Fail fast at startup if admin routes are enabled with
  an empty password.

- [x] **7. `RetryDeadJob` task-ID collision on repeated retries** — FIXED 2026-06-21
  `internal/usecase/admin/admin.go:110` hardcodes `id + "-retry"`. Retrying the
  same dead job twice reuses the same asynq TaskID → dedup conflict → re-enqueue
  fails. Use a unique suffix (attempt counter or timestamp).

- [x] **8. Webhook delivery doesn't drain/cap response body** — FIXED 2026-06-21
  `internal/worker/handlers/webhook_delivery.go:78` — `resp.Body` is closed but
  never drained/limited; large/slow bodies aren't bounded. Add
  `io.Copy(io.Discard, io.LimitReader(...))` before close for connection reuse.

---

## 🟢 Low — consistency / polish

- [x] **9. Inconsistent logging — `log.Printf` vs `slog`** — FIXED 2026-06-21
  README advertises structured logging everywhere, but
  `internal/usecase/job/job.go:124`, `internal/usecase/admin/admin.go:53`, and
  handlers `report.go`/`sms.go`/`webhook.go` still use stdlib `log.Printf`
  (no `request_id`/`job_id` context). Migrate to `logger.FromContext(ctx)`.

- [x] **10. Duplicated `jobTypeToTaskType`** — FIXED 2026-06-21
  Defined in both `internal/usecase/job/job.go:131` and
  `internal/usecase/admin/admin.go:133` (the admin copy hardcodes string literals
  instead of the `queue.Type*` consts). Consolidate into the `queue` package.

- [x] **11. `GetStats` mislabels asynq `Failed` as `Dead`** — FIXED 2026-06-21 (was a real bug: now uses `info.Archived`)
  `internal/usecase/admin/admin.go:72` maps `info.Failed` → `QueueStats.Dead`.
  asynq's "failed/archived" set isn't exactly your "dead" semantics; verify the
  label or rename for clarity.

- [x] **12. Hardcoded queue name `"default"`** — FIXED 2026-06-21 (now `queue.DefaultQueue`)
  `internal/usecase/job/job.go:123` (`DeleteTask`) and
  `internal/usecase/admin/admin.go:51` (`GetQueueInfo`) hardcode `"default"`.
  Make it a shared constant so it can't drift from the producer side.

---

### Suggested order

Tackle **#1–#4** first (high-severity) in one branch, then medium, then polish.

---

## 🚀 Upgrades (post-audit improvements)

> Identified 2026-06-21 after the 12 audit fixes landed. These are enhancements,
> not bug fixes. Ordered by value/risk.

- [x] **U1. API graceful shutdown** — DONE 2026-06-21
  `cmd/api/main.go` calls `router.Run(addr)` (blocking, no signal handling), so a
  deploy/restart cuts off in-flight HTTP requests. The worker already drains
  cleanly on SIGTERM — bring the API to parity by wrapping Gin in an
  `http.Server` and calling `srv.Shutdown(ctx)` on SIGTERM/SIGINT.

- [x] **U2. `go.mod` hygiene + dependency refresh** — DONE 2026-06-21
  Every entry in the `require` block is marked `// indirect`, even direct
  dependencies (gin, gorm, asynq, pgx). Run `go mod tidy` to reclassify, then
  refresh the available minor/patch bumps (pgx 5.9.2→5.10.0, go-redis
  9.19→9.20.1, validator 10.30.1→10.30.3, testcontainers 0.42→0.43, etc.).

- [x] **U3. Unit tests for the recent fixes** — DONE 2026-06-21
  Added: job use case (idempotent replay, enqueue rollback, max_retries clamp),
  webhook Delete error propagation, queue.TaskTypeForJob mapping, constant-time
  admin auth. Made JobUseCase depend on small Enqueuer/TaskCanceller interfaces
  so it's testable without a live Redis.

- [ ] **U4. CI / linting / vuln scanning**
  No GitHub Actions, no golangci-lint config, no govulncheck. Add a ci.yml
  running `go vet`, `go test`, `golangci-lint`, and `govulncheck`.

- [ ] **U5. Architectural (larger, optional)**
  - Enqueue durability: replace the best-effort create→enqueue→compensating-delete
    with a transactional outbox so a crash mid-enqueue can't lose/orphan work.
  - Retry bookkeeping: the handler's DB `retry_count` is tracked separately from
    asynq's own retry counter — they can drift. Unify them.
  - Schema migrations: GORM AutoMigrate works now; a real tool (goose/atlas) is
    the production-grade step.
  - Webhook secret stored in plaintext at rest — consider encrypting.
