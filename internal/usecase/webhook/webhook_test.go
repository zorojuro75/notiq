package webhook

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/pkg/apperror"
)

type mockWebhookRepo struct {
	deleteErr error
}

func (m *mockWebhookRepo) Create(ctx context.Context, w *entity.Webhook) error { return nil }
func (m *mockWebhookRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.Webhook, error) {
	return nil, apperror.ErrWebhookNotFound
}
func (m *mockWebhookRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Webhook, error) {
	return nil, nil
}
func (m *mockWebhookRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return m.deleteErr
}

func TestDelete_PropagatesNotFound(t *testing.T) {
	uc := NewWebhookUseCase(&mockWebhookRepo{deleteErr: apperror.ErrWebhookNotFound})
	err := uc.Delete(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, apperror.ErrWebhookNotFound) {
		t.Errorf("expected ErrWebhookNotFound, got %v", err)
	}
}

func TestDelete_DoesNotMaskRealErrors(t *testing.T) {
	dbErr := errors.New("connection refused")
	uc := NewWebhookUseCase(&mockWebhookRepo{deleteErr: dbErr})
	err := uc.Delete(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, dbErr) {
		t.Errorf("expected the underlying error to propagate, got %v", err)
	}
	if errors.Is(err, apperror.ErrWebhookNotFound) {
		t.Error("a real DB error must not be masked as ErrWebhookNotFound")
	}
}

func TestDelete_NilOnSuccess(t *testing.T) {
	uc := NewWebhookUseCase(&mockWebhookRepo{deleteErr: nil})
	if err := uc.Delete(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Errorf("expected nil on success, got %v", err)
	}
}
