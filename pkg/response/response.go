package response

import (
	"github.com/gofiber/fiber/v2"
)

type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

type ListResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Meta    Meta        `json:"meta"`
}

type Meta struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

type ErrorResponse struct {
	Success bool        `json:"success"`
	Error   ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func OK(c *fiber.Ctx, data interface{}, message ...string) error {
	msg := "Success"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	return c.Status(fiber.StatusOK).JSON(SuccessResponse{
		Success: true,
		Data:    data,
		Message: msg,
	})
}

func Created(c *fiber.Ctx, data interface{}, message ...string) error {
	msg := "Created successfully"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	return c.Status(fiber.StatusCreated).JSON(SuccessResponse{
		Success: true,
		Data:    data,
		Message: msg,
	})
}

func List(c *fiber.Ctx, data interface{}, page, limit int, total int64) error {
	return c.Status(fiber.StatusOK).JSON(ListResponse{
		Success: true,
		Data:    data,
		Meta: Meta{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	})
}

func Error(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(ErrorResponse{
		Success: false,
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

func BadRequest(c *fiber.Ctx, message string, code ...string) error {
	errCode := "BAD_REQUEST"
	if len(code) > 0 && code[0] != "" {
		errCode = code[0]
	}
	return Error(c, fiber.StatusBadRequest, errCode, message)
}

func NotFound(c *fiber.Ctx, message string, code ...string) error {
	errCode := "NOT_FOUND"
	if len(code) > 0 && code[0] != "" {
		errCode = code[0]
	}
	return Error(c, fiber.StatusNotFound, errCode, message)
}

func Unauthorized(c *fiber.Ctx, message string, code ...string) error {
	errCode := "UNAUTHORIZED"
	if len(code) > 0 && code[0] != "" {
		errCode = code[0]
	}
	return Error(c, fiber.StatusUnauthorized, errCode, message)
}

func Forbidden(c *fiber.Ctx, message string, code ...string) error {
	errCode := "FORBIDDEN"
	if len(code) > 0 && code[0] != "" {
		errCode = code[0]
	}
	return Error(c, fiber.StatusForbidden, errCode, message)
}

func InternalServerError(c *fiber.Ctx, message string, code ...string) error {
	errCode := "INTERNAL_SERVER_ERROR"
	if len(code) > 0 && code[0] != "" {
		errCode = code[0]
	}
	return Error(c, fiber.StatusInternalServerError, errCode, message)
}
