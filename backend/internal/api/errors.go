package api

import (
	"github.com/gofiber/fiber/v3"
)

const (
	// Error messages for request validation
	ErrInvalidRequestBody = "invalid request body"
	ErrInvalidJSON        = "invalid JSON"
	ErrMissingField       = "missing required field"
	ErrInvalidValue       = "invalid value"
)

// RespondWithError sends a JSON error response with the given message and status.
func RespondWithError(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}

// RespondWithServerError sends a JSON error response with the error message and 500 status.
func RespondWithServerError(c fiber.Ctx, err error) error {
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}

// RespondOK sends a JSON {"status":"ok"} response.
func RespondOK(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}
