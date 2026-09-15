package bowline

import (
	"context"
	"errors"
	"fmt"
)

type Error struct {
	Code    Code
	Message string
	Details any
	Issues  []Issue
	cause   error
}

type Issue struct {
	Path    []string `json:"path"`
	Rule    string   `json:"rule"`
	Message string   `json:"message"`
}

func Errorf(code Code, format string, args ...any) *Error {
	wrapped := fmt.Errorf(format, args...)
	return &Error{Code: code, Message: wrapped.Error(), cause: errors.Unwrap(wrapped)}
}

func (e *Error) Error() string {
	return string(e.Code) + ": " + e.Message
}

func (e *Error) Unwrap() error {
	return e.cause
}

func (e *Error) WithDetails(d any) *Error {
	e.Details = d
	return e
}

type wireError struct {
	Code    Code    `json:"code"`
	Message string  `json:"message"`
	Type    string  `json:"type,omitempty"`
	Details any     `json:"details,omitempty"`
	Issues  []Issue `json:"issues,omitempty"`
}

type wireEnvelope struct {
	Error wireError `json:"error"`
}

func classify(err error, production bool, variants []variant) (int, wireEnvelope, bool) {
	for _, v := range variants {
		if coded, value, ok := v.match(err); ok {
			return coded.Code().HTTPStatus(), v.envelope(coded, value), false
		}
	}
	var be *Error
	var coded Coded
	switch {
	case errors.As(err, &be):
		return be.Code.HTTPStatus(), wireEnvelope{wireError{Code: be.Code, Message: be.Message, Details: be.Details, Issues: be.Issues}}, false
	case errors.As(err, &coded):
		return coded.Code().HTTPStatus(), wireEnvelope{wireError{Code: coded.Code(), Message: coded.Error()}}, true
	case errors.Is(err, context.Canceled):
		return Canceled.HTTPStatus(), wireEnvelope{wireError{Code: Canceled, Message: "request canceled"}}, false
	case errors.Is(err, context.DeadlineExceeded):
		return DeadlineExceeded.HTTPStatus(), wireEnvelope{wireError{Code: DeadlineExceeded, Message: "deadline exceeded"}}, false
	}
	message := "internal error"
	if !production {
		message = err.Error()
	}
	return Internal.HTTPStatus(), wireEnvelope{wireError{Code: Internal, Message: message}}, false
}
