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

- [ ] **5. Admin basic-auth uses non-constant-time comparison**
  `internal/delivery/http/middleware/auth.go:16` — `u != username || p != password`
  is timing-attackable. Use `crypto/subtle.ConstantTimeCompare` / `hmac.Equal`.

- [ ] **6. No validation that `ADMIN_PASSWORD` is set**
  `config/config.go` reads it but never enforces it. An empty configured password
  is a weak admin surface. Fail fast at startup if admin routes are enabled with
  an empty password.

- [ ] **7. `RetryDeadJob` task-ID collision on repeated retries**
  `internal/usecase/admin/admin.go:110` hardcodes `id + "-retry"`. Retrying the
  same dead job twice reuses the same asynq TaskID → dedup conflict → re-enqueue
  fails. Use a unique suffix (attempt counter or timestamp).

- [ ] **8. Webhook delivery doesn't drain/cap response body**
  `internal/worker/handlers/webhook_delivery.go:78` — `resp.Body` is closed but
  never drained/limited; large/slow bodies aren't bounded. Add
  `io.Copy(io.Discard, io.LimitReader(...))` before close for connection reuse.

---

## 🟢 Low — consistency / polish

- [ ] **9. Inconsistent logging — `log.Printf` vs `slog`**
  README advertises structured logging everywhere, but
  `internal/usecase/job/job.go:124`, `internal/usecase/admin/admin.go:53`, and
  handlers `report.go`/`sms.go`/`webhook.go` still use stdlib `log.Printf`
  (no `request_id`/`job_id` context). Migrate to `logger.FromContext(ctx)`.

- [ ] **10. Duplicated `jobTypeToTaskType`**
  Defined in both `internal/usecase/job/job.go:131` and
  `internal/usecase/admin/admin.go:133` (the admin copy hardcodes string literals
  instead of the `queue.Type*` consts). Consolidate into the `queue` package.

- [ ] **11. `GetStats` mislabels asynq `Failed` as `Dead`**
  `internal/usecase/admin/admin.go:72` maps `info.Failed` → `QueueStats.Dead`.
  asynq's "failed/archived" set isn't exactly your "dead" semantics; verify the
  label or rename for clarity.

- [ ] **12. Hardcoded queue name `"default"`**
  `internal/usecase/job/job.go:123` (`DeleteTask`) and
  `internal/usecase/admin/admin.go:51` (`GetQueueInfo`) hardcode `"default"`.
  Make it a shared constant so it can't drift from the producer side.

---

### Suggested order

Tackle **#1–#4** first (high-severity) in one branch, then medium, then polish.
