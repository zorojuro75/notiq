# notiq

A background job queue and notification engine built in Go. Enqueue jobs via REST API — notiq handles reliable delivery with automatic retries, exponential backoff, and full status tracking.

---

## What it does

- Accepts jobs via REST API — email, SMS, webhook, report
- Processes jobs concurrently in the background (asynq-managed worker concurrency)
- Retries failed jobs with exponential backoff and jitter
- Tracks every job through its full lifecycle — pending → processing → done / failed / dead
- Delivers signed job lifecycle events to registered webhooks (HMAC-SHA256), each retried independently
- Guards all outbound webhook calls against SSRF (loopback / private / link-local targets blocked)
- Prevents duplicate jobs with idempotency keys
- Supports delayed and scheduled job execution
- Exposes Prometheus metrics at `/metrics` with a Grafana dashboard

---

## Architecture

```text
POST /api/v1/jobs
       ↓
Gin HTTP API → Postgres (job record) + Redis (asynq task)
                                            ↓
                          asynq worker (concurrency 10)
                                            ↓
                              Job handler (email / sms / webhook / report)
                                            ↓
                              Postgres status update (done / failed / dead)
                                            ↓
                       signed webhook event → owner's registered URLs
```

**Stack:** Go 1.25 · Gin · GORM · PostgreSQL · Redis · asynq · Prometheus · Grafana · Docker

---

## Project structure

```text
cmd/
  api/            HTTP server entry point
  worker/         Background worker entry point
internal/
  domain/
    entity/       Pure domain structs — Job, Webhook, DTOs
    contracts/    Use case interfaces
    repository/   Repository interfaces
  usecase/
    job/          Enqueue, GetByID, List, Cancel
    webhook/      Create, List, Delete
    notification/ HMAC-signed webhook dispatch
    admin/        Stats, manual retry
  delivery/http/
    handler/      Gin handlers — job, webhook, admin, health
    middleware/   RequestID, Logger, Metrics, BasicAuth
    router.go     Route registration
  repository/
    models/       GORM models
    postgres/     GORM implementations + integration tests
  worker/
    processor.go  asynq server — registers handlers, bounds concurrency, graceful drain
    handlers/     Email, SMS, webhook, report + webhook-delivery handlers
pkg/
  apperror/       Sentinel errors
  logger/         Structured logging (slog) + context propagation
  metrics/        Prometheus counters, histograms, gauges
  queue/          asynq client + inspector wrapper, task-type mapping
  response/       HTTP response helpers
  retry/          Exponential backoff with jitter
  safehttp/       SSRF-hardened HTTP client for outbound webhooks
  signature/      HMAC-SHA256 signing and verification
```

---

## Quick start

### Prerequisites

- Go 1.25+
- Docker + Docker Compose

### Run with Docker Compose

```bash
git clone https://github.com/yourusername/notiq.git
cd notiq

cp .env.example .env

docker compose up --build
```

Services started:

| Service    | URL                          |
|------------|------------------------------|
| API        | ```http://localhost:8080```  |
| Grafana    | ```http://localhost:3000```  |
| Prometheus | ```http://localhost:9090```  |

### Run locally without Docker

```bash
# start Postgres and Redis
docker run -d --name notiq-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=notiq \
  -p 5432:5432 postgres:16-alpine

docker run -d --name notiq-redis \
  -p 6379:6379 redis:7-alpine

# terminal 1 — API
go run ./cmd/api

# terminal 2 — worker
go run ./cmd/worker
```

---

## API reference

### Jobs

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/jobs` | Enqueue a new job |
| GET | `/api/v1/jobs` | List jobs |
| GET | `/api/v1/jobs/:id` | Get job by ID |
| DELETE | `/api/v1/jobs/:id` | Cancel a pending job |

> `max_retries` must be between `0` and `100` (defaults to `3`). Set an optional
> `user_id` (UUID) to have terminal job events delivered to that user's webhooks.

### Webhooks

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/webhooks` | Register a callback URL |
| GET | `/api/v1/webhooks` | List webhooks |
| DELETE | `/api/v1/webhooks/:id` | Remove a webhook |

### Admin (basic auth required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/admin/stats` | Queue stats |
| POST | `/api/v1/admin/jobs/:id/retry` | Retry a dead job |

### System

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/healthz` | Health check — Postgres + Redis |
| GET | `/metrics` | Prometheus metrics |

---

## Testing with Postman

### Setup

Base URL: `http://localhost:8080`

Create a Postman environment with variable `base_url = http://localhost:8080`.

---

### Health check

```code
Method:  GET
URL:     {{base_url}}/healthz
```

Expected response:

```json
{ "postgres": "ok", "redis": "ok" }
```

---

### Enqueue an email job

```cod
Method:  POST
URL:     {{base_url}}/api/v1/jobs
Headers: Content-Type: application/json
Body (raw JSON):
{
  "type": "email",
  "payload": {
    "to": "user@example.com",
    "subject": "Hello from notiq",
    "body": "Your background job queue works."
  },
  "max_retries": 3
}
```

Expected response — `201 Created`:

```json
{
  "success": true,
  "data": {
    "id": "abc-123",
    "type": "email",
    "status": "pending",
    "retry_count": 0,
    "max_retries": 3,
    "created_at": "2026-06-07T10:00:00Z",
    "updated_at": "2026-06-07T10:00:00Z"
  }
}
```

---

### Enqueue with idempotency key

```code
Method:  POST
URL:     {{base_url}}/api/v1/jobs
Headers:
  Content-Type: application/json
  X-Idempotency-Key: order-email-001
Body (raw JSON):
{
  "type": "email",
  "payload": { "to": "user@example.com", "subject": "Order confirmed" }
}
```

Send the same request again with the same `X-Idempotency-Key`. The second response returns `200 OK` with header `X-Idempotent-Replayed: true` and the same job ID — no duplicate created.

---

### Enqueue a scheduled job

```code
Method:  POST
URL:     {{base_url}}/api/v1/jobs
Headers: Content-Type: application/json
Body (raw JSON):
{
  "type": "report",
  "payload": { "report_type": "monthly_sales" },
  "max_retries": 1,
  "scheduled_at": "2026-06-08T09:00:00Z"
}
```

The job sits in Redis's scheduled set until the specified time, then moves to the pending queue automatically.

---

### Enqueue an SMS job

```code
Method:  POST
URL:     {{base_url}}/api/v1/jobs
Headers: Content-Type: application/json
Body (raw JSON):
{
  "type": "sms",
  "payload": {
    "phone": "+8801700000000",
    "message": "Your OTP is 482910"
  },
  "max_retries": 5
}
```

---

### Enqueue a webhook job

```code
Method:  POST
URL:     {{base_url}}/api/v1/jobs
Headers: Content-Type: application/json
Body (raw JSON):
{
  "type": "webhook",
  "payload": {
    "url": "https://webhook.site/your-unique-id",
    "event": "order.completed",
    "data": { "order_id": "ORD-999" }
  },
  "max_retries": 3
}
```

---

### Get job by ID

```code
Method:  GET
URL:     {{base_url}}/api/v1/jobs/{{job_id}}
```

Replace `{{job_id}}` with the ID returned from the enqueue response.

---

### List all jobs

```code
Method:  GET
URL:     {{base_url}}/api/v1/jobs
```

---

### List jobs with filters

Filter by status:

```code
Method:  GET
URL:     {{base_url}}/api/v1/jobs?status=pending
```

Filter by type:

```code
Method:  GET
URL:     {{base_url}}/api/v1/jobs?type=email
```

Filter scheduled jobs only:

```code
Method:  GET
URL:     {{base_url}}/api/v1/jobs?scheduled=true
```

Combined filters with pagination:

```code
Method:  GET
URL:     {{base_url}}/api/v1/jobs?status=failed&type=email&page=1&page_size=10
```

---

### Cancel a pending job

```code
Method:  DELETE
URL:     {{base_url}}/api/v1/jobs/{{job_id}}
```

Only works on jobs with `status: pending`. Returns `409 Conflict` if the job is already processing, done, or dead.

---

### Register a webhook URL

```code
Method:  POST
URL:     {{base_url}}/api/v1/webhooks
Headers: Content-Type: application/json
Body (raw JSON):
{
  "url": "https://webhook.site/your-unique-id",
  "user_id": "00000000-0000-0000-0000-000000000001"
}
```

The response includes a `secret` — store it safely. It will never be shown again. Use it to verify the `X-Notiq-Signature` header on incoming deliveries.

---

### List webhooks

```code
Method:  GET
URL:     {{base_url}}/api/v1/webhooks?user_id=00000000-0000-0000-0000-000000000001
```

---

### Delete a webhook

```code
Method:  DELETE
URL:     {{base_url}}/api/v1/webhooks/{{webhook_id}}?user_id=00000000-0000-0000-0000-000000000001
```

---

### Admin — queue stats

```code
Method:  GET
URL:     {{base_url}}/api/v1/admin/stats
Auth:    Basic Auth
  Username: admin
  Password: notiq-admin-secret
```

In Postman: Authorization tab → Type: Basic Auth → fill in username and password.

Expected response:

```json
{
  "success": true,
  "data": {
    "queue": {
      "pending": 5,
      "active": 2,
      "retry": 1,
      "dead": 0,
      "scheduled": 3,
      "completed": 142
    },
    "dead_jobs_in_db": 0
  }
}
```

---

### Admin — manually retry a dead job

```code
Method:  POST
URL:     {{base_url}}/api/v1/admin/jobs/{{job_id}}/retry
Auth:    Basic Auth
  Username: admin
  Password: notiq-admin-secret
```

Only works on jobs with `status: dead`. Resets `retry_count` to 0 and re-enqueues the job.

---

## Job lifecycle

```text
pending     job created, waiting in Redis queue
    ↓
processing  worker claimed it, handler running
    ↓
done        handler succeeded
    ↓ (on failure)
failed      handler errored, retry scheduled with backoff
    ↓ (after max retries exhausted)
dead        needs manual intervention via admin API

cancelled   DELETE /jobs/:id called before worker picked it up
```

Retry delays use exponential backoff with jitter:

| Attempt | Approximate delay |
|---------|-------------------|
| 1 | ~2s |
| 2 | ~4s |
| 3 | ~8s |
| 4 | ~16s |
| max | 5 minutes |

---

## Webhook signing

When a job reaches a terminal state (`done` or `dead`), notiq delivers a signed
event to every webhook its owner has registered (set `user_id` on the job and
register URLs via `POST /api/v1/webhooks`). Each delivery is enqueued as its own
asynq task with independent exponential-backoff retries, so a slow or failing
subscriber never blocks job processing. Outbound calls go through the
SSRF-hardened client.

Every delivery includes an HMAC-SHA256 signature header:

```code
X-Notiq-Signature: sha256=a1b2c3d4...
```

Verify in your receiver:

```go
body, _ := io.ReadAll(r.Body)
sig := r.Header.Get("X-Notiq-Signature")
valid := signature.Verify(yourSecret, body, sig)
if !valid {
    http.Error(w, "invalid signature", 401)
    return
}
```

---

## Observability

**Structured logs** — every line includes `request_id` (API) or `job_id` (worker). JSON format in production.

```json
{"level":"INFO","msg":"job enqueued","request_id":"550e8400","job_id":"abc-123","type":"email"}
{"level":"WARN","msg":"retry scheduled","job_id":"abc-123","attempt":1,"next_in":"4.2s"}
{"level":"INFO","msg":"job completed","job_id":"abc-123","status":"done","duration_ms":234}
```

**Prometheus metrics at `/metrics`:**

| Metric | Type | Description |
|--------|------|-------------|
| `notiq_jobs_enqueued_total` | Counter | Jobs enqueued by type |
| `notiq_jobs_processed_total` | Counter | Jobs processed by type and status |
| `notiq_job_processing_duration_seconds` | Histogram | Processing time by type |
| `notiq_http_requests_total` | Counter | HTTP requests by method, path, status |
| `notiq_http_request_duration_seconds` | Histogram | HTTP latency by method and path |

Grafana dashboard at `http://localhost:3000` — login: `admin / admin`.

---

## Running tests

```bash
# unit tests only — no Docker needed
go test ./... -short

# all tests including integration — requires Docker
go test ./... -v

# integration tests only
go test ./internal/repository/postgres/... -v -run TestIntegration

# with race detector
go test ./pkg/... -v -race
```

Integration tests use `testcontainers-go` — each test spins up its own isolated Postgres and Redis container and destroys it after. No shared state, no manual setup.

---

## Configuration

Copy `.env.example` to `.env` and fill in values:

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_PORT` | HTTP server port | `8080` |
| `DB_HOST` | Postgres host | `localhost` |
| `DB_PORT` | Postgres port | `5432` |
| `DB_USER` | Postgres user | `postgres` |
| `DB_PASSWORD` | Postgres password | — |
| `DB_NAME` | Database name | `notiq` |
| `DB_SSLMODE` | Postgres SSL mode (disable/require/...) | `disable` |
| `DB_LOG_LEVEL` | GORM log level (silent/warn/info) | `warn` |
| `REDIS_ADDR` | Redis address | `localhost:6379` |
| `REDIS_PASSWORD` | Redis password | — |
| `REDIS_DB` | Redis database number | `0` |
| `WORKER_SHUTDOWN_TIMEOUT` | Graceful shutdown window | `30s` |
| `ADMIN_USERNAME` | Admin basic auth username (**required** — API won't start if empty) | `admin` |
| `ADMIN_PASSWORD` | Admin basic auth password (**required** — API won't start if empty) | — |
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `LOG_FORMAT` | Log format (text/json) | `text` |
| `WEBHOOK_ALLOW_PRIVATE` | Disable the outbound SSRF guard — **dev only**, never in production | `false` |

---

## What I learned building this

- Clean architecture in Go — domain, use cases, delivery, and infrastructure with no framework bleed
- Reliable background job processing with asynq and Redis — bounded concurrency and graceful drain on SIGTERM
- Exponential backoff with jitter (overflow-safe) to prevent thundering herd
- Idempotency keys to prevent duplicate job creation under network failure — with rollback so a failed enqueue never orphans a row
- HMAC-SHA256 webhook signing with constant-time comparison against timing attacks
- SSRF-hardened outbound HTTP — a dialer Control hook blocks loopback/private/link-local IPs even after DNS resolution
- Fan-out webhook delivery — each subscriber notification is its own retryable task, so a slow receiver never blocks job processing
- Structured logging with `slog` — request ID and job ID propagated through `context.Context`
- Prometheus instrumentation — counters, histograms, gauges with correct label cardinality
- Integration testing with `testcontainers-go` — real Postgres and Redis, no mocks for persistence
- Multi-stage Docker builds — ~20MB runtime image from an 800MB builder
- Graceful shutdown — asynq drains in-flight handlers within the timeout, then re-queues anything still running

---

## Author

**Banna** — Associate Software Engineer, DataEdge Limited
CSE, Independent University Bangladesh
[GitHub](https://github.com/yourusername)
