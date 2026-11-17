package domain

import (
	"fmt"
	"net/http"
)

// Standard Error Codes specified in PRD §6
const (
	CodeValidationError   = "VALIDATION_ERROR"
	CodeNotFound          = "NOT_FOUND"
	CodeDuplicate         = "DUPLICATE"
	CodeUnauthorized      = "UNAUTHORIZED"
	CodeForbidden         = "FORBIDDEN"
	CodeInsufficientStock = "INSUFFICIENT_STOCK"
	CodeUnsupportedFile   = "UNSUPPORTED_FILE"
	CodeFileTooLarge      = "FILE_TOO_LARGE"
	CodeInternal          = "INTERNAL"
)

// AppError is the domain error representing API errors with HTTP status and structured details.
type AppError struct {
	Code       string      `json:"code"`
	Message    string      `json:"message"`
	HTTPStatus int         `json:"-"`
	Details    interface{} `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	if e.Details != nil {
		return fmt.Sprintf("[%s] %s (status: %d, details: %+v)", e.Code, e.Message, e.HTTPStatus, e.Details)
	}
	return fmt.Sprintf("[%s] %s (status: %d)", e.Code, e.Message, e.HTTPStatus)
}

// ValidationErrorDetail represents a single field validation failure
type ValidationErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// InsufficientStockDetail represents stock deficit per product
type InsufficientStockDetail struct {
	ProductCode string `json:"kode_barang"`
	Requested   int64  `json:"diminta"`
	Available   int64  `json:"tersedia"`
}

func NewAppError(httpStatus int, code, message string, details interface{}) *AppError {
	return &AppError{
		HTTPStatus: httpStatus,
		Code:       code,
		Message:    message,
		Details:    details,
	}
}

func NewNotFoundError(message string) *AppError {
	return NewAppError(http.StatusNotFound, CodeNotFound, message, nil)
}

func NewDuplicateError(message string) *AppError {
	return NewAppError(http.StatusConflict, CodeDuplicate, message, nil)
}

func NewUnauthorizedError(message string) *AppError {
	return NewAppError(http.StatusUnauthorized, CodeUnauthorized, message, nil)
}

func NewForbiddenError(message string) *AppError {
	return NewAppError(http.StatusForbidden, CodeForbidden, message, nil)
}

func NewValidationError(message string, details ...ValidationErrorDetail) *AppError {
	return NewAppError(http.StatusUnprocessableEntity, CodeValidationError, message, details)
}

func NewInsufficientStockError(message string, details ...InsufficientStockDetail) *AppError {
	return NewAppError(http.StatusConflict, CodeInsufficientStock, message, details)
}

func NewUnsupportedFileError(message string) *AppError {
	return NewAppError(http.StatusUnsupportedMediaType, CodeUnsupportedFile, message, nil)
}

func NewFileTooLargeError(message string) *AppError {
	return NewAppError(http.StatusRequestEntityTooLarge, CodeFileTooLarge, message, nil)
}

func NewInternalError(message string) *AppError {
	return NewAppError(http.StatusInternalServerError, CodeInternal, message, nil)
}

