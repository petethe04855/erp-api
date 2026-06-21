package handlers

import (
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
	result := database.DB.Where("id = ? OR name = ?", req.Username, req.Username).First(&user)
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
		"userId": user.ID,
		"name":   user.Name,
		"role":   user.Role,
		"exp":    time.Now().Add(time.Hour * 24).Unix(),
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
			"name":        user.Name,
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
		"name":        user.Name,
		"role":        user.Role,
		"isActive":    user.IsActive,
		"lastLoginAt": user.LastLoginAt,
	})
}

var validRoles = map[string]bool{
	"owner": true, "sales": true, "warehouse": true, "accountant": true,
}

func validateUserFields(name, role, password string, passwordRequired bool) string {
	if strings.TrimSpace(name) == "" {
		return "Display name is required"
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
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

// CreateUser handles registration of new accounts
func CreateUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid body"})
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	if req.ID == "" || req.Name == "" || req.Role == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "All fields are required"})
	}
	if message := validateUserFields(req.Name, req.Role, req.Password, true); message != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": message})
	}

	var count int64
	database.DB.Model(&models.AppUser{}).Where("id = ?", req.ID).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User ID already exists"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	user := models.AppUser{
		ID:       req.ID,
		Name:     req.Name,
		Role:     req.Role,
		Password: string(hashedPassword),
		IsActive: true,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(user)
}

type UpdateUserRequest struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
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

	nextName := user.Name
	if strings.TrimSpace(req.Name) != "" {
		nextName = strings.TrimSpace(req.Name)
	}
	nextRole := user.Role
	if req.Role != "" {
		nextRole = req.Role
	}
	if message := validateUserFields(nextName, nextRole, req.Password, false); message != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": message})
	}
	if user.Role == "owner" && nextRole != "owner" {
		var ownerCount int64
		database.DB.Model(&models.AppUser{}).Where("role = ? AND is_active = ?", "owner", true).Count(&ownerCount)
		if ownerCount <= 1 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "The last active owner cannot be demoted"})
		}
	}

	if req.Name != "" {
		user.Name = req.Name
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
