package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type apiSuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type apiErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiErrorResponse struct {
	Success bool           `json:"success"`
	Error   apiErrorDetail `json:"error"`
}

// StandardizeJSONResponse wraps API JSON responses in a stable envelope.
// File exports and other non-JSON responses pass through unchanged.
func StandardizeJSONResponse(c *fiber.Ctx) error {
	if err := c.Next(); err != nil {
		return err
	}

	if !strings.HasPrefix(c.Path(), "/api/") ||
		!strings.Contains(strings.ToLower(string(c.Response().Header.ContentType())), "application/json") {
		return nil
	}

	body := c.Response().Body()
	if len(body) == 0 {
		return nil
	}

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}

	if object, ok := payload.(map[string]interface{}); ok {
		if _, alreadyWrapped := object["success"]; alreadyWrapped && (object["data"] != nil || object["error"] != nil) {
			return nil
		}
	}

	status := c.Response().StatusCode()
	if status >= fiber.StatusBadRequest {
		message := http.StatusText(status)
		if object, ok := payload.(map[string]interface{}); ok {
			if value, ok := object["error"].(string); ok && value != "" {
				message = value
			}
		}
		code := strings.ToUpper(strings.ReplaceAll(http.StatusText(status), " ", "_"))
		return c.Status(status).JSON(apiErrorResponse{
			Success: false,
			Error:   apiErrorDetail{Code: code, Message: message},
		})
	}

	return c.Status(status).JSON(apiSuccessResponse{Success: true, Data: payload})
}
