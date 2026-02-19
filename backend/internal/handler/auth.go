package handler

import (
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/naratel/naratel-box/backend/internal/auth"
	"github.com/naratel/naratel-box/backend/internal/repository"
)

// RegisterRequest is the payload for POST /auth/register.
type RegisterRequest struct {
	Email    string `json:"email"    example:"user@example.com"`
	Password string `json:"password" example:"supersecret123"`
}

// LoginRequest is the payload for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"    example:"user@example.com"`
	Password string `json:"password" example:"supersecret123"`
}

// TokenResponse is returned on successful login.
type TokenResponse struct {
	Token     string `json:"token"      example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	ExpiresAt string `json:"expires_at" example:"2026-02-19T10:00:00Z"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error   string `json:"error"   example:"unauthorized"`
	Message string `json:"message" example:"invalid email or password"`
}

type AuthHandler struct {
	userRepo       *repository.UserRepository
	jwtSecret      string
	jwtExpiryHours int
}

func NewAuthHandler(userRepo *repository.UserRepository, jwtSecret string, jwtExpiryHours int) *AuthHandler {
	return &AuthHandler{
		userRepo:       userRepo,
		jwtSecret:      jwtSecret,
		jwtExpiryHours: jwtExpiryHours,
	}
}

// Register godoc
// @Summary      Register a new user
// @Description  Create a new account with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body     RegisterRequest true "Register payload"
// @Success      201  {object} map[string]interface{} "user_id and email"
// @Failure      400  {object} ErrorResponse
// @Failure      409  {object} ErrorResponse
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "bad_request",
			"message": "invalid request body",
		})
	}
	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "bad_request",
			"message": "email and password are required",
		})
	}
	if len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "bad_request",
			"message": "password must be at least 8 characters",
		})
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_error"})
	}

	user, err := h.userRepo.Create(c.Context(), req.Email, string(hashed))
	if err != nil {
		// Naive duplicate check — pgx will surface unique constraint error
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":   "conflict",
			"message": "email already registered",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user_id": user.ID,
		"email":   user.Email,
	})
}

// Login godoc
// @Summary      Login
// @Description  Authenticate with email and password, receive a JWT token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body     LoginRequest  true "Login payload"
// @Success      200  {object} TokenResponse
// @Failure      400  {object} ErrorResponse
// @Failure      401  {object} ErrorResponse
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "bad_request",
			"message": "invalid request body",
		})
	}

	user, err := h.userRepo.FindByEmail(c.Context(), req.Email)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   "unauthorized",
			"message": "invalid email or password",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   "unauthorized",
			"message": "invalid email or password",
		})
	}

	token, expiresAt, err := auth.GenerateToken(user.ID, user.Email, h.jwtSecret, h.jwtExpiryHours)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal_error"})
	}

	return c.JSON(fiber.Map{
		"token":      token,
		"expires_at": expiresAt,
	})
}
