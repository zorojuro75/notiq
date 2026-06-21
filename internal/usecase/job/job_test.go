package job

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/queue"
)

// ── test doubles ──────────────────────────────────────────────────────────────

type mockJobRepo struct {
	createFn    func(ctx context.Context, job *entity.Job) error
	getByKeyFn  func(ctx context.Context, key string) (*entity.Job, error)
	deleteFn    func(ctx context.Context, id uuid.UUID) error
	deletedIDs  []uuid.UUID
	createdJobs []*entity.Job
}

func (m *mockJobRepo) Create(ctx context.Context, job *entity.Job) error {
	m.createdJobs = append(m.createdJobs, job)
	if m.createFn != nil {
		return m.createFn(ctx, job)
	}
	return nil
}

func (m *mockJobRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.Job, error) {
	return nil, apperror.ErrJobNotFound
}

func (m *mockJobRepo) GetByIdempotencyKey(ctx context.Context, key string) (*entity.Job, error) {
	if m.getByKeyFn != nil {
		return m.getByKeyFn(ctx, key)
	}
	return nil, apperror.ErrJobNotFound
}

func (m *mockJobRepo) List(ctx context.Context, f entity.JobFilter, page, size int) ([]*entity.Job, int64, error) {
	return nil, 0, nil
}

func (m *mockJobRepo) UpdateStatus(ctx context.Context, id uuid.UUID, s entity.JobStatus) error {
	return nil
}

func (m *mockJobRepo) UpdateRetryCount(ctx context.Context, id uuid.UUID, c int) error {
	return nil
}

func (m *mockJobRepo) Delete(ctx context.Context, id uuid.UUID) error {
	m.deletedIDs = append(m.deletedIDs, id)
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

type mockEnqueuer struct {
	err    error
	called int
}

func (m *mockEnqueuer) Enqueue(taskType string, payload any, opts queue.EnqueueOptions) error {
	m.called++
	return m.err
}

type noopCanceller struct{}

func (noopCanceller) DeleteTask(queueName, taskID string) error { return nil }

func newUC(repo *mockJobRepo, q *mockEnqueuer) *JobUseCase {
	return NewJobUseCase(repo, q, noopCanceller{})
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestEnqueue_IdempotentReplay(t *testing.T) {
	existing := &entity.Job{ID: uuid.New(), Type: entity.JobTypeEmail}
	repo := &mockJobRepo{
		getByKeyFn: func(ctx context.Context, key string) (*entity.Job, error) {
			return existing, nil
		},
	}
	q := &mockEnqueuer{}
	uc := newUC(repo, q)

	key := "dup-key"
	out, err := uc.Enqueue(context.Background(), entity.EnqueueJobInput{
		Type:           entity.JobTypeEmail,
		Payload:        []byte(`{}`),
		IdempotencyKey: &key,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Replayed {
		t.Error("expected Replayed=true for an existing idempotency key")
	}
	if out.Job.ID != existing.ID {
		t.Errorf("expected existing job %s, got %s", existing.ID, out.Job.ID)
	}
	if q.called != 0 {
		t.Errorf("queue should not be touched on replay, called %d times", q.called)
	}
	if len(repo.createdJobs) != 0 {
		t.Error("no job should be created on replay")
	}
}

func TestEnqueue_RollsBackJobWhenQueueFails(t *testing.T) {
	repo := &mockJobRepo{}
	q := &mockEnqueuer{err: errors.New("redis down")}
	uc := newUC(repo, q)

	_, err := uc.Enqueue(context.Background(), entity.EnqueueJobInput{
		Type:    entity.JobTypeEmail,
		Payload: []byte(`{}`),
	})
	if err == nil {
		t.Fatal("expected an error when enqueue fails")
	}
	if len(repo.createdJobs) != 1 {
		t.Fatalf("expected one created job, got %d", len(repo.createdJobs))
	}
	if len(repo.deletedIDs) != 1 {
		t.Fatalf("expected the orphaned job to be deleted, got %d deletes", len(repo.deletedIDs))
	}
	if repo.deletedIDs[0] != repo.createdJobs[0].ID {
		t.Error("the rolled-back delete must target the created job")
	}
}

func TestEnqueue_ClampsMaxRetries(t *testing.T) {
	cases := map[string]int{
		"zero defaults to 3":     0,
		"negative defaults to 3": -5,
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &mockJobRepo{}
			q := &mockEnqueuer{}
			uc := newUC(repo, q)

			_, err := uc.Enqueue(context.Background(), entity.EnqueueJobInput{
				Type:       entity.JobTypeEmail,
				Payload:    []byte(`{}`),
				MaxRetries: in,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := repo.createdJobs[0].MaxRetries; got != 3 {
				t.Errorf("expected MaxRetries clamped to 3, got %d", got)
			}
		})
	}
}

func TestEnqueue_KeepsValidMaxRetries(t *testing.T) {
	repo := &mockJobRepo{}
	q := &mockEnqueuer{}
	uc := newUC(repo, q)

	_, err := uc.Enqueue(context.Background(), entity.EnqueueJobInput{
		Type:       entity.JobTypeEmail,
		Payload:    []byte(`{}`),
		MaxRetries: 7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.createdJobs[0].MaxRetries; got != 7 {
		t.Errorf("expected MaxRetries 7 preserved, got %d", got)
	}
}
