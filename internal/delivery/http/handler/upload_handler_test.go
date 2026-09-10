package handler_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"chawy-erp-api/internal/delivery/http/handler"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestUploadHandler_UploadImage(t *testing.T) {
	app := fiber.New(fiber.Config{
		BodyLimit: 10 * 1024 * 1024,
	})
	uploadHdl := handler.NewUploadHandler()
	app.Post("/upload/image", uploadHdl.UploadImage)

	defer os.RemoveAll("./uploads")

	t.Run("success upload png", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("image", "sample.png")
		assert.NoError(t, err)
		_, _ = part.Write([]byte("fake png content"))
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/upload/image", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("reject invalid file extension", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("image", "script.exe")
		assert.NoError(t, err)
		_, _ = part.Write([]byte("fake executable"))
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/upload/image", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("reject file exceeding 5MB", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("image", "huge.jpg")
		assert.NoError(t, err)
		// 5MB + 1KB
		hugeData := make([]byte, 5*1024*1024+1024)
		_, _ = part.Write(hugeData)
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/upload/image", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
