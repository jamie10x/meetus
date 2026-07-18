// Package apperr defines typed application errors that services return
// and the HTTP layer maps to status codes.
package apperr

import "fmt"

type Code string

const (
	CodeValidation   Code = "validation_error"
	CodeUnauthorized Code = "unauthorized"
	CodeForbidden    Code = "forbidden"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeInternal     Code = "internal_error"
)

type Error struct {
	Code    Code
	Message string
	// Err is the underlying cause, kept for logs and errors.Is/As chains.
	Err error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Wrap(code Code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

func Validation(message string) *Error   { return New(CodeValidation, message) }
func Unauthorized(message string) *Error { return New(CodeUnauthorized, message) }
func Forbidden(message string) *Error    { return New(CodeForbidden, message) }
func NotFound(message string) *Error     { return New(CodeNotFound, message) }
func Conflict(message string) *Error     { return New(CodeConflict, message) }

func Internal() *Error { return New(CodeInternal, "something went wrong") }
