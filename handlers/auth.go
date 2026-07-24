package handlers

import (
	"fmt"
	"math/rand"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login verifies credentials and returns a signed JWT token
func Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid JSON body",
		})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username and password are required",
		})
	}

	var user models.AppUser
	username := strings.TrimSpace(req.Username)
	result := database.DB.Where("id = ? OR name = ? OR email = ?", username, username, strings.ToLower(username)).First(&user)
	if result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid username or password",
		})
	}
	if !user.IsActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "This account has been disabled"})
	}

	// Compare passwords
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid username or password",
		})
	}

	// Generate JWT Token
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "supersecretjwtkeyforchawyerp2026"
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId":    user.ID,
		"firstname": user.Firstname,
		"lastname":  user.Lastname,
		"role":      user.Role,
		"exp":       time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	now := time.Now()
	database.DB.Model(&user).Update("last_login_at", &now)
	user.LastLoginAt = &now

	return c.JSON(fiber.Map{
		"token": tokenString,
		"user": fiber.Map{
			"id":          user.ID,
			"email":       user.Email,
			"firstname":   user.Firstname,
			"lastname":    user.Lastname,
			"role":        user.Role,
			"isActive":    user.IsActive,
			"lastLoginAt": user.LastLoginAt,
		},
	})
}

// GetCurrentUser returns the user info of the currently logged-in user
func GetCurrentUser(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	var user models.AppUser
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"id":          user.ID,
		"email":       user.Email,
		"firstname":   user.Firstname,
		"lastname":    user.Lastname,
		"role":        user.Role,
		"isActive":    user.IsActive,
		"lastLoginAt": user.LastLoginAt,
	})
}

var validRoles = map[string]bool{
	"owner": true, "sales": true, "warehouse": true, "accountant": true,
}

func validateUserFields(firstname, lastname, email, role, password string, emailRequired, passwordRequired bool) string {
	if strings.TrimSpace(firstname) == "" || strings.TrimSpace(lastname) == "" {
		return "Display name is required"
	}
	if emailRequired || strings.TrimSpace(email) != "" {
		if !isRealEmailAddress(email) {
			return "A valid email address is required"
		}
	}
	if !validRoles[role] {
		return "Invalid user role"
	}
	if (passwordRequired || password != "") && len(password) < 8 {
		return "Password must be at least 8 characters"
	}
	return ""
}

type CreateUserRequest struct {
	Email     string `json:"email"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Role      string `json:"role"`
	Password  string `json:"password"`
}

// CreateUser handles registration of new accounts
func CreateUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid body"})
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Firstname = strings.TrimSpace(req.Firstname)
	req.Lastname = strings.TrimSpace(req.Lastname)

	if req.Password == "" {
		const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%"
		b := make([]byte, 10)
		for i := range b {
			b[i] = chars[rand.Intn(len(chars))]
		}
		req.Password = string(b)
	}
	if req.Email == "" || req.Firstname == "" || req.Lastname == "" || req.Role == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "All fields are required"})
	}
	if message := validateUserFields(req.Firstname, req.Lastname, req.Email, req.Role, req.Password, true, true); message != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": message})
	}

	var count int64
	database.DB.Model(&models.AppUser{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email already exists"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	user := models.AppUser{
		Email:     req.Email,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      req.Role,
		Password:  string(hashedPassword),
		IsActive:  true,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := sendUserCreatedEmail(user, req.Password); err != nil {
		user.EmailWarning = fmt.Sprintf("ส่งอีเมลแจ้งเตือนไม่สำเร็จ: %v", err)
	}

	return c.JSON(user)
}

type UpdateUserRequest struct {
	Email     string `json:"email"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Role      string `json:"role"`
	Password  string `json:"password"`
}

// UpdateUser handles updating display name, role, or password of an account
func UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid body"})
	}

	var user models.AppUser
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	nextFirstname := user.Firstname
	if strings.TrimSpace(req.Firstname) != "" {
		nextFirstname = strings.TrimSpace(req.Firstname)
	}
	nextLastname := user.Lastname
	if strings.TrimSpace(req.Lastname) != "" {
		nextLastname = strings.TrimSpace(req.Lastname)
	}
	nextEmail := user.Email
	if strings.TrimSpace(req.Email) != "" {
		nextEmail = strings.ToLower(strings.TrimSpace(req.Email))
	}
	nextRole := user.Role
	if req.Role != "" {
		nextRole = req.Role
	}
	if message := validateUserFields(nextFirstname, nextLastname, nextEmail, nextRole, req.Password, false, false); message != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": message})
	}
	if nextEmail != user.Email {
		var count int64
		database.DB.Model(&models.AppUser{}).Where("email = ? AND id <> ?", nextEmail, user.ID).Count(&count)
		if count > 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email already exists"})
		}
	}
	if user.Role == "owner" && nextRole != "owner" {
		var ownerCount int64
		database.DB.Model(&models.AppUser{}).Where("role = ? AND is_active = ?", "owner", true).Count(&ownerCount)
		if ownerCount <= 1 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "The last active owner cannot be demoted"})
		}
	}

	if req.Firstname != "" {
		user.Firstname = req.Firstname
	}
	if req.Lastname != "" {
		user.Lastname = req.Lastname
	}
	if strings.TrimSpace(req.Email) != "" {
		user.Email = nextEmail
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
		}
		user.Password = string(hashedPassword)
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(user)
}

type UpdateUserStatusRequest struct {
	IsActive *bool `json:"isActive"`
}

// UpdateUserStatus enables or disables an account without deleting its history.
func UpdateUserStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req UpdateUserStatusRequest
	if err := c.BodyParser(&req); err != nil || req.IsActive == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "isActive is required"})
	}

	currentUserID, _ := c.Locals("userID").(string)
	if id == currentUserID && !*req.IsActive {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "You cannot disable your own account"})
	}

	var user models.AppUser
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}
	if user.Role == "owner" && !*req.IsActive && user.IsActive {
		var ownerCount int64
		database.DB.Model(&models.AppUser{}).Where("role = ? AND is_active = ?", "owner", true).Count(&ownerCount)
		if ownerCount <= 1 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "The last active owner cannot be disabled"})
		}
	}

	user.IsActive = *req.IsActive
	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(user)
}

// DeleteUser permanently removes an account while protecting the current user and last owner.
func DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	currentUserID, _ := c.Locals("userID").(string)
	if id == currentUserID {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "You cannot delete your own account"})
	}

	var user models.AppUser
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}
	if user.Role == "owner" && user.IsActive {
		var ownerCount int64
		database.DB.Model(&models.AppUser{}).Where("role = ? AND is_active = ?", "owner", true).Count(&ownerCount)
		if ownerCount <= 1 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "The last active owner cannot be deleted"})
		}
	}

	if err := database.DB.Delete(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"id": id})
}

func isRealEmailAddress(value string) bool {
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil || address.Address != strings.TrimSpace(value) {
		return false
	}
	parts := strings.Split(address.Address, "@")
	if len(parts) != 2 || parts[0] == "" || !strings.Contains(parts[1], ".") {
		return false
	}
	_, err = net.LookupMX(parts[1])
	return err == nil
}

func sendUserCreatedEmail(user models.AppUser, plainPassword string) error {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	username := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	password := os.Getenv("SMTP_PASSWORD")
	from := strings.TrimSpace(os.Getenv("EMAIL_SEND"))
	if host == "" || port == "" || username == "" || password == "" || from == "" {
		return fmt.Errorf("SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, and EMAIL_SEND must be configured")
	}

	body := fmt.Sprintf("Hello %s %s,\r\n\r\nYour Chawy ERP account has been created.\r\n\r\nUsername: %s\r\nEmail: %s\r\nTemporary password: %s\r\n\r\nPlease sign in and change your password.\r\n", user.Firstname, user.Lastname, user.ID, user.Email, plainPassword)
	message := strings.Join([]string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", user.Email),
		"Subject: Your Chawy ERP account is ready",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	auth := smtp.PlainAuth("", username, password, host)
	return smtp.SendMail(net.JoinHostPort(host, port), auth, from, []string{user.Email}, []byte(message))
}
