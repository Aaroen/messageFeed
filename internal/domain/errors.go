package domain

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrRateLimited  = errors.New("rate limited")
)

type ErrorKind string

const (
	ErrorKindInvalidInput ErrorKind = "invalid_input"
	ErrorKindNotFound     ErrorKind = "not_found"
	ErrorKindConflict     ErrorKind = "conflict"
	ErrorKindRateLimited  ErrorKind = "rate_limited"
	ErrorKindUnavailable  ErrorKind = "unavailable"
	ErrorKindInternal     ErrorKind = "internal"
)

type AppError struct {
	Kind      ErrorKind
	Code      string
	Message   string
	Operation string
	Retryable bool
	Err       error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Message
	}
	if e.Message == "" {
		return e.Err.Error()
	}
	return e.Message + ": " + e.Err.Error()
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *AppError) Is(target error) bool {
	if e == nil {
		return false
	}
	switch target {
	case ErrInvalidInput:
		return e.Kind == ErrorKindInvalidInput
	case ErrNotFound:
		return e.Kind == ErrorKindNotFound
	case ErrConflict:
		return e.Kind == ErrorKindConflict
	case ErrRateLimited:
		return e.Kind == ErrorKindRateLimited
	default:
		return false
	}
}

func NewAppError(kind ErrorKind, code string, message string, operation string, retryable bool, err error) *AppError {
	return &AppError{
		Kind:      kind,
		Code:      code,
		Message:   message,
		Operation: operation,
		Retryable: retryable,
		Err:       err,
	}
}

func ClassifyError(err error) ErrorKind {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Kind
	}
	switch {
	case errors.Is(err, ErrInvalidInput):
		return ErrorKindInvalidInput
	case errors.Is(err, ErrNotFound):
		return ErrorKindNotFound
	case errors.Is(err, ErrConflict):
		return ErrorKindConflict
	case errors.Is(err, ErrRateLimited):
		return ErrorKindRateLimited
	default:
		return ErrorKindInternal
	}
}
