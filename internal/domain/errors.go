package domain

import "fmt"

type ErrorCode string

const (
	CodeValidation       ErrorCode = "validation_error"
	CodeNotFound         ErrorCode = "not_found"
	CodeConflict         ErrorCode = "version_conflict"
	CodeInvalidState     ErrorCode = "invalid_state"
	CodeForbidden        ErrorCode = "forbidden"
	CodeIntegrity        ErrorCode = "integrity_error"
	CodeIdempotencyReuse ErrorCode = "idempotency_key_reused"
)

type RuleError struct {
	Code    ErrorCode
	Message string
}

func (e *RuleError) Error() string { return e.Message }

func NewRuleError(code ErrorCode, format string, args ...any) error {
	return &RuleError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func ErrorCodeOf(err error) ErrorCode {
	if value, ok := err.(*RuleError); ok {
		return value.Code
	}
	return CodeIntegrity
}
