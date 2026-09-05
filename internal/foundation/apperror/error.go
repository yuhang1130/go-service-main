package apperror

import (
	"errors"
	"net/http"
)

type Error struct {
	Code       string
	Message    string
	HTTPStatus int
	Cause      error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func Internal(cause error) *Error {
	return &Error{Code: "INTERNAL_ERROR", Message: "internal server error", HTTPStatus: http.StatusInternalServerError, Cause: cause}
}

func InvalidArgument(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: http.StatusBadRequest, Cause: cause}
}

func Unauthorized(code, message string) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: http.StatusUnauthorized}
}

func Forbidden(code, message string) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: http.StatusForbidden}
}

func NotFound(code, message string) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: http.StatusNotFound}
}

func Conflict(code, message string) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: http.StatusConflict}
}

func As(err error) *Error {
	var target *Error
	if errors.As(err, &target) {
		return target
	}
	return Internal(err)
}
