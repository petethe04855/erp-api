package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UploadHandler struct{}

func NewUploadHandler() *UploadHandler {
	return &UploadHandler{}
}

// UploadImage handles image uploads (png, jpg, jpeg) under 5MB
func (h *UploadHandler) UploadImage(c *fiber.Ctx) error {
	file, err := c.FormFile("image")
	if err != nil {
		file, err = c.FormFile("file")
		if err != nil {
			return response.BadRequest(c, "กรุณาเลือกไฟล์รูปภาพที่ต้องการอัปโหลด (image หรือ file)")
		}
	}

	// 1. Validate file size (max 5MB = 5 * 1024 * 1024 bytes)
	const maxFileSize = 5 * 1024 * 1024
	if file.Size > maxFileSize {
		return response.BadRequest(c, "ขนาดไฟล์รูปภาพเกินกำหนด (ต้องไม่เกิน 5 MB)")
	}

	// 2. Validate file extension (only .png, .jpg, .jpeg)
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return response.BadRequest(c, "รองรับเฉพาะไฟล์รูปภาพนามสกุล .png, .jpg หรือ .jpeg เท่านั้น")
	}

	// 3. Ensure uploads/images directory exists
	uploadDir := "./uploads/images"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return response.InternalServerError(c, "ไม่สามารถสร้างโฟลเดอร์สำหรับเก็บรูปภาพได้: "+err.Error())
	}

	// 4. Generate unique filename
	filename := fmt.Sprintf("%d-%s%s", time.Now().Unix(), uuid.New().String()[:8], ext)
	dst := filepath.Join(uploadDir, filename)

	// 5. Save file to disk
	if err := c.SaveFile(file, dst); err != nil {
		return response.InternalServerError(c, "บันทึกไฟล์รูปภาพไม่สำเร็จ: "+err.Error())
	}

	// 6. Return relative URL path
	imageURL := "/uploads/images/" + filename
	return response.Created(c, fiber.Map{
		"url":      imageURL,
		"filename": filename,
		"size":     file.Size,
	}, "อัปโหลดรูปภาพสำเร็จ")
}
