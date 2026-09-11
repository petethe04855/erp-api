package handler

import (
	"fmt"
	"io"
	"net/http"
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

// allowedImageMimes lists the content types accepted for image uploads.
// Sniffing is done with http.DetectContentType on the first 512 bytes so a
// text file renamed to .png/.jpg is rejected.
var allowedImageMimes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
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

	// 3. Sniff real content type from the file header — extension alone is not
	// trusted (a text file renamed to .png must be rejected).
	f, err := file.Open()
	if err != nil {
		return response.BadRequest(c, "ไม่สามารถอ่านไฟล์รูปภาพได้")
	}
	defer f.Close()

	head := make([]byte, 512)
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		return response.BadRequest(c, "ไม่สามารถอ่านไฟล์รูปภาพได้")
	}
	contentType := http.DetectContentType(head[:n])
	normalized := strings.Split(contentType, ";")[0]
	if normalized == "image/jpg" {
		normalized = "image/jpeg"
	}
	expectedExt, ok := allowedImageMimes[normalized]
	if !ok {
		return response.BadRequest(c, "เนื้อหาไฟล์ไม่ใช่รูปภาพ PNG หรือ JPEG ที่ถูกต้อง")
	}
	// Extension must agree with actual content (.jpeg aliases to .jpg).
	if ext != ".png" && ext != ".jpeg" && ext != expectedExt {
		return response.BadRequest(c, "นามสกุลไฟล์ไม่ตรงกับเนื้อหารูปภาพจริง")
	}

	// Rewind so the saved file is complete.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return response.InternalServerError(c, "ไม่สามารถอ่านไฟล์รูปภาพได้")
	}

	// 4. Ensure uploads/images directory exists
	uploadDir := "./uploads/images"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return response.InternalServerError(c, "ไม่สามารถสร้างโฟลเดอร์สำหรับเก็บรูปภาพได้: "+err.Error())
	}

	// 5. Generate unique filename
	filename := fmt.Sprintf("%d-%s%s", time.Now().Unix(), uuid.New().String()[:8], ext)
	dst := filepath.Join(uploadDir, filename)

	// 6. Save file to disk
	if err := c.SaveFile(file, dst); err != nil {
		return response.InternalServerError(c, "บันทึกไฟล์รูปภาพไม่สำเร็จ: "+err.Error())
	}

	// 7. Return relative URL path
	imageURL := "/uploads/images/" + filename
	return response.Created(c, fiber.Map{
		"url":      imageURL,
		"filename": filename,
		"size":     file.Size,
	}, "อัปโหลดรูปภาพสำเร็จ")
}
