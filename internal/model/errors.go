package model

import "fmt"

// ErrorCode is a business error code returned to agents.
type ErrorCode string

const (
	ErrPluginNotFound  ErrorCode = "PLUGIN_NOT_FOUND"
	ErrInvalidParams   ErrorCode = "INVALID_PARAMS"
	ErrPluginTimeout   ErrorCode = "PLUGIN_TIMEOUT"
	ErrConnectorError  ErrorCode = "CONNECTOR_ERROR"
	ErrRuntimeError    ErrorCode = "RUNTIME_ERROR"
	ErrInternalError   ErrorCode = "INTERNAL_ERROR"
)

// AppError is a structured business error.
type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// ToolResult is the unified business result shape.
type ToolResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *AppError   `json:"error,omitempty"`
}

func SuccessResult(data interface{}) ToolResult {
	return ToolResult{Success: true, Data: data}
}

func FailResult(code ErrorCode, message string) ToolResult {
	return ToolResult{
		Success: false,
		Error:   NewAppError(code, message),
	}
}
