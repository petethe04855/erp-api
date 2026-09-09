package middleware

import (
	"errors"

	appErrors "chawy-erp-api/pkg/errors"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

func GlobalErrorHandler(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}

	// 1. Fiber standard error (*fiber.Error)
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return response.Error(c, fiberErr.Code, "HTTP_ERROR", fiberErr.Message)
	}

	// 2. Custom AppError
	var appErr *appErrors.AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.StatusCode, appErr.Code, appErr.Message)
	}

	// 3. Known Domain Errors
	switch {
	case errors.Is(err, appErrors.ErrNotFound), errors.Is(err, appErrors.ErrSKUNotFound):
		return response.NotFound(c, err.Error(), "NOT_FOUND")
	case errors.Is(err, appErrors.ErrUnauthorized), errors.Is(err, appErrors.ErrInvalidCredentials):
		return response.Unauthorized(c, err.Error(), "UNAUTHORIZED")
	case errors.Is(err, appErrors.ErrForbidden):
		return response.Forbidden(c, err.Error(), "FORBIDDEN")
	case errors.Is(err, appErrors.ErrValidation):
		return response.BadRequest(c, err.Error(), "VALIDATION_ERROR")
	case errors.Is(err, appErrors.ErrConflict), errors.Is(err, appErrors.ErrSKUAlreadyExists), errors.Is(err, appErrors.ErrUserAlreadyExists):
		return response.Error(c, fiber.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, appErrors.ErrInsufficientStock):
		return response.BadRequest(c, err.Error(), "INSUFFICIENT_STOCK")
	case errors.Is(err, appErrors.ErrInvalidOrderStatus):
		return response.BadRequest(c, err.Error(), "INVALID_ORDER_STATUS")
	default:
		return response.InternalServerError(c, "An unexpected error occurred", "INTERNAL_SERVER_ERROR")
	}
}
