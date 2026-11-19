package httpx

import (
	"github.com/gofiber/fiber/v2"
)

// ErrorResponse represents the structured error payload (§6)
type ErrorResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// APIResponse represents the standard JSON envelope response required by PRD §6
type APIResponse struct {
	Success bool           `json:"success"`
	Data    interface{}    `json:"data"`
	Meta    interface{}    `json:"meta,omitempty"`
	Error   *ErrorResponse `json:"error"`
}

// WriteSuccess responds with HTTP status, wrapped data, and optional metadata
func WriteSuccess(c *fiber.Ctx, statusCode int, data interface{}, meta interface{}) error {
	return c.Status(statusCode).JSON(APIResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
		Error:   nil,
	})
}

// WriteOK responds with HTTP 200 OK
func WriteOK(c *fiber.Ctx, data interface{}) error {
	return WriteSuccess(c, fiber.StatusOK, data, nil)
}

// WriteOKWithMeta responds with HTTP 200 OK and metadata
func WriteOKWithMeta(c *fiber.Ctx, data interface{}, meta interface{}) error {
	return WriteSuccess(c, fiber.StatusOK, data, meta)
}

// WriteCreated responds with HTTP 201 Created
func WriteCreated(c *fiber.Ctx, data interface{}) error {
	return WriteSuccess(c, fiber.StatusCreated, data, nil)
}

// WriteAccepted responds with HTTP 202 Accepted
func WriteAccepted(c *fiber.Ctx, data interface{}) error {
	return WriteSuccess(c, fiber.StatusAccepted, data, nil)
}

// WriteError responds with HTTP error matching PRD envelope
func WriteError(c *fiber.Ctx, statusCode int, code string, message string, details interface{}) error {
	return c.Status(statusCode).JSON(APIResponse{
		Success: false,
		Data:    nil,
		Error: &ErrorResponse{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

