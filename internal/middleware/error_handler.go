package middleware

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/mongo"

	"smartcoop/internal/domain"
	"smartcoop/internal/httpx"
)

// ErrorHandler is the global error handler middleware for Go Fiber
func ErrorHandler(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	// 1. Check if error is our custom domain.AppError
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return httpx.WriteError(c, appErr.HTTPStatus, appErr.Code, appErr.Message, appErr.Details)
	}

	// 2. Check if error is a Fiber standard error (e.g., 404 Route Not Found, 413 Body Limit)
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code := domain.CodeInternal
		switch fiberErr.Code {
		case fiber.StatusNotFound:
			code = domain.CodeNotFound
		case fiber.StatusRequestEntityTooLarge:
			code = domain.CodeFileTooLarge
		case fiber.StatusUnsupportedMediaType:
			code = domain.CodeUnsupportedFile
		case fiber.StatusUnprocessableEntity:
			code = domain.CodeValidationError
		case fiber.StatusConflict:
			code = domain.CodeDuplicate
		}
		return httpx.WriteError(c, fiberErr.Code, code, fiberErr.Message, nil)
	}

	// 3. Check for MongoDB specific errors
	if errors.Is(err, mongo.ErrNoDocuments) {
		return httpx.WriteError(c, http.StatusNotFound, domain.CodeNotFound, "Data tidak ditemukan", nil)
	}

	// Check for Mongo duplicate key error (code 11000)
	if isMongoDuplicateKeyError(err) {
		return httpx.WriteError(c, http.StatusConflict, domain.CodeDuplicate, "Data dengan kunci tersebut sudah ada", nil)
	}

	// 4. Fallback for unhandled unexpected internal errors
	log.Printf("[ERROR] Unhandled Internal Error: %v\n", err)
	return httpx.WriteError(
		c,
		http.StatusInternalServerError,
		domain.CodeInternal,
		"Terjadi kesalahan internal pada server",
		nil,
	)
}

func isMongoDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	var writeException mongo.WriteException
	if errors.As(err, &writeException) {
		for _, we := range writeException.WriteErrors {
			if we.Code == 11000 {
				return true
			}
		}
	}
	return strings.Contains(err.Error(), "E11000")
}

