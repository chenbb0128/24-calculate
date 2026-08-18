package apperror

import (
	"errors"
	"net/http"
)

const (
	CodeBadRequest         = 10001
	CodeNotFound           = 10002
	CodeConflict           = 10003
	CodeForbidden          = 10004
	CodeInternal           = 50000
	CodeServiceUnavailable = 50001
)

type AppError struct {
	Code       int
	HTTPStatus int
	Message    string
	Err        error
}

func (e *AppError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(code, status int, message string, err error) *AppError {
	return &AppError{Code: code, HTTPStatus: status, Message: message, Err: err}
}

func BadRequest(message string, err error) *AppError {
	return New(CodeBadRequest, http.StatusBadRequest, message, err)
}

func NotFound(message string, err error) *AppError {
	return New(CodeNotFound, http.StatusNotFound, message, err)
}

func Conflict(message string, err error) *AppError {
	return New(CodeConflict, http.StatusConflict, message, err)
}

func ServiceUnavailable(message string, err error) *AppError {
	return New(CodeServiceUnavailable, http.StatusServiceUnavailable, message, err)
}

func Internal(err error) *AppError {
	return New(CodeInternal, http.StatusInternalServerError, "服务器内部错误", err)
}

func From(err error) *AppError {
	if err == nil {
		return Internal(nil)
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal(err)
}
