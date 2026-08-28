// Package domain defines Sandrone's internal models, requests, reports, and error types.
package domain

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeInvalidArgument        ErrorCode = "invalid_argument"
	CodeInputKindAmbiguous     ErrorCode = "input_kind_ambiguous"
	CodeInputKindUnrecognized  ErrorCode = "input_kind_unrecognized"
	CodeParseFailed            ErrorCode = "parse_failed"
	CodeRenderFailed           ErrorCode = "render_failed"
	CodeFileInputNotFound      ErrorCode = "file_input_not_found"
	CodeFileMergeFailed        ErrorCode = "file_merge_failed"
	CodeFileProcessorFailed    ErrorCode = "file_processor_failed"
	CodeNotImplemented         ErrorCode = "not_implemented"
	CodeProcessorUnknown       ErrorCode = "processor_unknown"
	CodeProcessorConfigInvalid ErrorCode = "processor_config_invalid"
	CodeNodeProcessorFailed    ErrorCode = "node_processor_failed"
	CodeNodeValidationFailed   ErrorCode = "node_validation_failed"
	CodeScriptTimeout          ErrorCode = "script_timeout"
	CodeScriptRuntime          ErrorCode = "script_runtime"
	CodeFileDependencyCycle    ErrorCode = "file_dependency_cycle"
	CodeBackupInvalid          ErrorCode = "backup_invalid"
	CodeBackupIncompatible     ErrorCode = "backup_incompatible"
	CodeBackupTooLarge         ErrorCode = "backup_too_large"
	CodeBackupRestoreFailed    ErrorCode = "backup_restore_failed"
	CodeCacheOperationFailed   ErrorCode = "cache_operation_failed"
)

type AppError struct {
	Code      ErrorCode `json:"code" yaml:"code"`
	Message   string    `json:"message" yaml:"message"`
	Source    string    `json:"source,omitempty" yaml:"source,omitempty"`
	Target    string    `json:"target,omitempty" yaml:"target,omitempty"`
	File      string    `json:"file,omitempty" yaml:"file,omitempty"`
	Part      string    `json:"part,omitempty" yaml:"part,omitempty"`
	Processor string    `json:"processor,omitempty" yaml:"processor,omitempty"`
	Path      string    `json:"path,omitempty" yaml:"path,omitempty"`
	Cause     error     `json:"-" yaml:"-"`
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewError(code ErrorCode, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func WrapError(code ErrorCode, message string, cause error) *AppError {
	return &AppError{Code: code, Message: message, Cause: cause}
}

func IsCode(err error, code ErrorCode) bool {
	if appErr, ok := errors.AsType[*AppError](err); ok {
		return appErr.Code == code
	}
	return false
}
