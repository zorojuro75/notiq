package apperror

import "errors"

var (
	ErrJobNotFound             = errors.New("job not found")
	ErrJobNotCancellable       = errors.New("job cannot be cancelled in its current state")
	ErrJobCancelled            = errors.New("job was cancelled")
	ErrWebhookNotFound         = errors.New("webhook not found")
	ErrWebhookUnauthorized     = errors.New("webhook not found")
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
)