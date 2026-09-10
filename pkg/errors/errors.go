package errors

import "errors"

// Common Domain Errors
var (
	ErrNotFound           = errors.New("resource not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrValidation         = errors.New("validation error")
	ErrConflict           = errors.New("resource conflict")
	ErrInternalServer     = errors.New("internal server error")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrSKUAlreadyExists   = errors.New("sku already exists")
	ErrSKUNotFound        = errors.New("sku not found")
	ErrInsufficientStock  = errors.New("insufficient stock")
	ErrInvalidOrderStatus = errors.New("invalid order status")
	ErrAccountDisabled    = errors.New("this account has been disabled")
	ErrCannotModifySelf   = errors.New("cannot disable or delete your own account")
	ErrInvalidRole        = errors.New("invalid user role")
)

// AppError is an error with machine-readable code and HTTP status code
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(code, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}
