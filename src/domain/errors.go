package domain

import "fmt"

type Code string

const (
	CodeInvalidRequest       Code = "invalid_request"
	CodeNotFound             Code = "not_found"
	CodePermissionDenied     Code = "permission_denied"
	CodeLimitExceeded        Code = "limit_exceeded"
	CodeInsufficientFunds    Code = "insufficient_funds"
	CodeApprovalRequired     Code = "approval_required"
	CodeApprovalRejected     Code = "approval_rejected"
	CodeSignerInactive       Code = "signer_inactive"
	CodeSignerConflict       Code = "signer_conflict"
	CodeOperationRejected    Code = "operation_rejected"
	CodeAccountBlocked       Code = "account_blocked"
	CodeExposureExceeded     Code = "exposure_exceeded"
	CodeUnknownAction        Code = "unknown_action"
	CodeInvariantViolation   Code = "invariant_violation"
	CodeRotationWindowClosed Code = "rotation_window_closed"
)

type Error struct {
	Code    Code              `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e Error) Error() string {
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewError(code Code, message string) Error {
	return Error{Code: code, Message: message}
}

func NewErrorf(code Code, format string, args ...any) Error {
	return Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func WithField(err Error, key string, value string) Error {
	if err.Fields == nil {
		err.Fields = map[string]string{}
	}
	err.Fields[key] = value
	return err
}

func Invalid(message string) Error {
	return NewError(CodeInvalidRequest, message)
}

func Invalidf(format string, args ...any) Error {
	return NewErrorf(CodeInvalidRequest, format, args...)
}

func NotFound(message string) Error {
	return NewError(CodeNotFound, message)
}

func NotFoundf(format string, args ...any) Error {
	return NewErrorf(CodeNotFound, format, args...)
}

func Permission(message string) Error {
	return NewError(CodePermissionDenied, message)
}

func Permissionf(format string, args ...any) Error {
	return NewErrorf(CodePermissionDenied, format, args...)
}

func Limit(message string) Error {
	return NewError(CodeLimitExceeded, message)
}

func Limitf(format string, args ...any) Error {
	return NewErrorf(CodeLimitExceeded, format, args...)
}

func Funds(message string) Error {
	return NewError(CodeInsufficientFunds, message)
}

func Fundsf(format string, args ...any) Error {
	return NewErrorf(CodeInsufficientFunds, format, args...)
}

func ApprovalRequired(message string) Error {
	return NewError(CodeApprovalRequired, message)
}

func ApprovalRequiredf(format string, args ...any) Error {
	return NewErrorf(CodeApprovalRequired, format, args...)
}

func ApprovalRejected(message string) Error {
	return NewError(CodeApprovalRejected, message)
}

func ApprovalRejectedf(format string, args ...any) Error {
	return NewErrorf(CodeApprovalRejected, format, args...)
}

func SignerInactive(message string) Error {
	return NewError(CodeSignerInactive, message)
}

func SignerInactivef(format string, args ...any) Error {
	return NewErrorf(CodeSignerInactive, format, args...)
}

func SignerConflict(message string) Error {
	return NewError(CodeSignerConflict, message)
}

func SignerConflictf(format string, args ...any) Error {
	return NewErrorf(CodeSignerConflict, format, args...)
}

func OperationRejectError(message string) Error {
	return NewError(CodeOperationRejected, message)
}

func OperationRejectErrorf(format string, args ...any) Error {
	return NewErrorf(CodeOperationRejected, format, args...)
}

func AccountBlocked(message string) Error {
	return NewError(CodeAccountBlocked, message)
}

func AccountBlockedf(format string, args ...any) Error {
	return NewErrorf(CodeAccountBlocked, format, args...)
}

func Exposure(message string) Error {
	return NewError(CodeExposureExceeded, message)
}

func Exposuref(format string, args ...any) Error {
	return NewErrorf(CodeExposureExceeded, format, args...)
}

func Invariant(message string) Error {
	return NewError(CodeInvariantViolation, message)
}

func Invariantf(format string, args ...any) Error {
	return NewErrorf(CodeInvariantViolation, format, args...)
}

func IsCode(err error, code Code) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(Error); ok {
		return e.Code == code
	}
	if e, ok := err.(*Error); ok {
		return e.Code == code
	}
	return false
}
