package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/repository/models"
	"github.com/zorojuro75/notiq/pkg/queue"
)

// Test 1 — happy path: enqueue → verify in DB → verify in Redis
func TestIntegration_EnqueueJob_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := setupPostgres(t)
	asynqClient, _ := setupRedis(t)

	jobRepo := NewJobRepository(db)

	payload, _ := json.Marshal(map[string]string{
		"to":      "test@example.com",
		"subject": "Integration test",
	})

	job := &entity.Job{
		ID:         uuid.New(),
		Type:       entity.JobTypeEmail,
		Payload:    payload,
		Status:     entity.JobStatusPending,
		MaxRetries: 3,
	}

	// step 1 — save to Postgres
	if err := jobRepo.Create(ctx, job); err != nil {
		t.Fatalf("creating job: %v", err)
	}

	// step 2 — verify in Postgres
	fetched, err := jobRepo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("fetching job: %v", err)
	}

	if fetched.Status != entity.JobStatusPending {
		t.Errorf("expected status pending, got %s", fetched.Status)
	}
	if fetched.Type != entity.JobTypeEmail {
		t.Errorf("expected type email, got %s", fetched.Type)
	}

	// step 3 — push to Redis
	task, err := asynqClient.Enqueue(
		asynq.NewTask(queue.TypeEmail, payload),
		asynq.TaskID(job.ID.String()),
	)
	if err != nil {
		t.Fatalf("enqueuing task: %v", err)
	}
	if task.ID != job.ID.String() {
		t.Errorf("expected task ID %s, got %s", job.ID, task.ID)
	}

	// step 4 — simulate worker: update status to done
	if err := jobRepo.UpdateStatus(ctx, job.ID, entity.JobStatusDone); err != nil {
		t.Fatalf("updating status: %v", err)
	}

	// step 5 — verify final state
	final, err := jobRepo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("fetching final job: %v", err)
	}
	if final.Status != entity.JobStatusDone {
		t.Errorf("expected status done, got %s", final.Status)
	}

	t.Logf("✓ job %s completed successfully", job.ID)
}

// Test 2 — retry on failure: fail → increment retry → dead on exhaustion
func TestIntegration_JobRetry_DeadOnExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := setupPostgres(t)

	jobRepo := NewJobRepository(db)

	payload, _ := json.Marshal(map[string]string{"to": "fail@example.com"})

	job := &entity.Job{
		ID:         uuid.New(),
		Type:       entity.JobTypeEmail,
		Payload:    payload,
		Status:     entity.JobStatusPending,
		MaxRetries: 3,
		RetryCount: 0,
	}

	if err := jobRepo.Create(ctx, job); err != nil {
		t.Fatalf("creating job: %v", err)
	}

	// simulate 3 failures — each increments retry count
	for attempt := 1; attempt <= 3; attempt++ {
		// update to processing
		if err := jobRepo.UpdateStatus(ctx, job.ID, entity.JobStatusProcessing); err != nil {
			t.Fatalf("attempt %d: updating to processing: %v", attempt, err)
		}

		// increment retry count
		if err := jobRepo.UpdateRetryCount(ctx, job.ID, attempt); err != nil {
			t.Fatalf("attempt %d: updating retry count: %v", attempt, err)
		}

		if attempt < 3 {
			// not last attempt — mark failed
			if err := jobRepo.UpdateStatus(ctx, job.ID, entity.JobStatusFailed); err != nil {
				t.Fatalf("attempt %d: updating to failed: %v", attempt, err)
			}

			mid, _ := jobRepo.GetByID(ctx, job.ID)
			if mid.Status != entity.JobStatusFailed {
				t.Errorf("attempt %d: expected failed, got %s", attempt, mid.Status)
			}
			if mid.RetryCount != attempt {
				t.Errorf("attempt %d: expected retry_count %d, got %d", attempt, attempt, mid.RetryCount)
			}
		} else {
			// last attempt — mark dead
			if err := jobRepo.UpdateStatus(ctx, job.ID, entity.JobStatusDead); err != nil {
				t.Fatalf("marking job dead: %v", err)
			}
		}
	}

	// verify final state
	dead, err := jobRepo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("fetching dead job: %v", err)
	}
	if dead.Status != entity.JobStatusDead {
		t.Errorf("expected dead, got %s", dead.Status)
	}
	if dead.RetryCount != 3 {
		t.Errorf("expected retry_count 3, got %d", dead.RetryCount)
	}

	t.Logf("✓ job %s correctly went dead after %d retries", job.ID, dead.RetryCount)
}

// Test 3 — idempotency: same key returns same job, no duplicate created
func TestIntegration_IdempotencyKey_NoDuplicate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	db := setupPostgres(t)

	jobRepo := NewJobRepository(db)

	key := "idempotency-test-key-" + uuid.New().String()
	payload, _ := json.Marshal(map[string]string{"to": "idem@example.com"})

	// first request — create job
	job1 := &entity.Job{
		ID:             uuid.New(),
		Type:           entity.JobTypeEmail,
		Payload:        payload,
		Status:         entity.JobStatusPending,
		MaxRetries:     3,
		IdempotencyKey: &key,
	}

	if err := jobRepo.Create(ctx, job1); err != nil {
		t.Fatalf("creating first job: %v", err)
	}

	// second request — same key, should return existing job
	existing, err := jobRepo.GetByIdempotencyKey(ctx, key)
	if err != nil {
		t.Fatalf("getting by idempotency key: %v", err)
	}

	if existing.ID != job1.ID {
		t.Errorf("expected same job ID %s, got %s", job1.ID, existing.ID)
	}

	// third request — try to create another job with same key
	// should fail with unique constraint violation
	job2 := &entity.Job{
		ID:             uuid.New(), // different ID
		Type:           entity.JobTypeEmail,
		Payload:        payload,
		Status:         entity.JobStatusPending,
		MaxRetries:     3,
		IdempotencyKey: &key, // same key
	}

	err = jobRepo.Create(ctx, job2)
	if err == nil {
		t.Error("expected error creating duplicate idempotency key, got nil")
	}

	// verify only one job exists with this key
	var count int64
	db.Model(&models.Job{}).Where("idempotency_key = ?", key).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 job with this key, found %d", count)
	}

	t.Logf("✓ idempotency key correctly prevented duplicate job creation")
}