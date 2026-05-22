package apperror

import "errors"

var (
    ErrJobNotFound       = errors.New("job not found")
    ErrJobNotCancellable = errors.New("job cannot be cancelled in its current state")
    ErrJobCancelled      = errors.New("job was cancelled")
)