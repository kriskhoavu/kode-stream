package common

import "errors"

type ErrorCode string

const (
	ErrorCodeNotFound     ErrorCode = "not_found"
	ErrorCodeValidation   ErrorCode = "validation"
	ErrorCodeConflict     ErrorCode = "conflict"
	ErrorCodeUnauthorized ErrorCode = "unauthorized"
	ErrorCodeForbidden    ErrorCode = "forbidden"
	ErrorCodeUnavailable  ErrorCode = "unavailable"
	ErrorCodeInfra        ErrorCode = "infra"
)

type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Code)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

var (
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrItemNotFound      = errors.New("item not found")
)

func NotFound(message string, err error) error {
	return &AppError{Code: ErrorCodeNotFound, Message: message, Err: err}
}

func Validation(message string, err error) error {
	return &AppError{Code: ErrorCodeValidation, Message: message, Err: err}
}

func Conflict(message string, err error) error {
	return &AppError{Code: ErrorCodeConflict, Message: message, Err: err}
}

func Unauthorized(message string, err error) error {
	return &AppError{Code: ErrorCodeUnauthorized, Message: message, Err: err}
}

func Forbidden(message string, err error) error {
	return &AppError{Code: ErrorCodeForbidden, Message: message, Err: err}
}

func Unavailable(message string, err error) error {
	return &AppError{Code: ErrorCodeUnavailable, Message: message, Err: err}
}

func Infra(message string, err error) error {
	return &AppError{Code: ErrorCodeInfra, Message: message, Err: err}
}
