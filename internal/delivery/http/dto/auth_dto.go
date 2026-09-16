package dto

import "time"

type RegisterRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Name      string `json:"name"`
	Role      string `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID          uint       `json:"id"`
	Email       string     `json:"email"`
	Firstname   string     `json:"firstname"`
	Lastname    string     `json:"lastname"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	IsActive    bool       `json:"isActive"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	CreatedAt   time.Time  `json:"created_at"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type CreateUserRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Name      string `json:"name"`
	Role      string `json:"role"`
}

type UpdateUserRequest struct {
	Email     *string `json:"email"`
	Password  *string `json:"password"`
	Firstname *string `json:"firstname"`
	Lastname  *string `json:"lastname"`
	Name      *string `json:"name"`
	Role      *string `json:"role"`
	IsActive  *bool   `json:"isActive"`
}

type UpdateUserStatusRequest struct {
	IsActive bool `json:"isActive"`
}
