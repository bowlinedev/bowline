package bowline

import "net/http"

type Code string

const (
	Canceled           Code = "CANCELED"
	Unknown            Code = "UNKNOWN"
	InvalidArgument    Code = "INVALID_ARGUMENT"
	DeadlineExceeded   Code = "DEADLINE_EXCEEDED"
	NotFound           Code = "NOT_FOUND"
	AlreadyExists      Code = "ALREADY_EXISTS"
	PermissionDenied   Code = "PERMISSION_DENIED"
	ResourceExhausted  Code = "RESOURCE_EXHAUSTED"
	FailedPrecondition Code = "FAILED_PRECONDITION"
	Aborted            Code = "ABORTED"
	OutOfRange         Code = "OUT_OF_RANGE"
	Unimplemented      Code = "UNIMPLEMENTED"
	Internal           Code = "INTERNAL"
	Unavailable        Code = "UNAVAILABLE"
	DataLoss           Code = "DATA_LOSS"
	Unauthenticated    Code = "UNAUTHENTICATED"
)

var httpStatus = map[Code]int{
	Canceled:           http.StatusRequestTimeout,
	Unknown:            http.StatusInternalServerError,
	InvalidArgument:    http.StatusBadRequest,
	DeadlineExceeded:   http.StatusRequestTimeout,
	NotFound:           http.StatusNotFound,
	AlreadyExists:      http.StatusConflict,
	PermissionDenied:   http.StatusForbidden,
	ResourceExhausted:  http.StatusTooManyRequests,
	FailedPrecondition: http.StatusPreconditionFailed,
	Aborted:            http.StatusConflict,
	OutOfRange:         http.StatusBadRequest,
	Unimplemented:      http.StatusNotFound,
	Internal:           http.StatusInternalServerError,
	Unavailable:        http.StatusServiceUnavailable,
	DataLoss:           http.StatusInternalServerError,
	Unauthenticated:    http.StatusUnauthorized,
}

func (c Code) HTTPStatus() int {
	if s, ok := httpStatus[c]; ok {
		return s
	}
	return http.StatusInternalServerError
}
