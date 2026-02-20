package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/naratel/naratel-box/backend/internal/auth"
	"github.com/naratel/naratel-box/backend/internal/repository"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

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
	Token     string    `json:"token"      example:"eyJhbGciOiJIUzI1NiJ9..."`
	ExpiresAt time.Time `json:"expires_at" example:"2026-02-19T10:00:00Z"`
}

// UserResponse is returned for profile endpoints.
type UserResponse struct {
	UserID    int64     `json:"user_id"    example:"5"`
	Email     string    `json:"email"      example:"user@example.com"`
	CreatedAt time.Time `json:"created_at" example:"2026-02-18T12:00:00Z"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error   string `json:"error"   example:"unauthorized"`
	Message string `json:"message" example:"invalid email or password"`
}

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	userRepo       *repository.UserRepository
	jwtSecret      string
	jwtExpiryHours int
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(userRepo *repository.UserRepository, jwtSecret string, jwtExpiryHours int) *AuthHandler {
	return &AuthHandler{
		userRepo:       userRepo,
		jwtSecret:      jwtSecret,
		jwtExpiryHours: jwtExpiryHours,
	}
}

// writeJSON is a helper that writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Register godoc
// @Summary      Register a new user
// @Description  Create a new account with email and password (minimum 8 characters)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body     RegisterRequest true "Register payload"
// @Success      201  {object} UserResponse
// @Failure      400  {object} ErrorResponse
// @Failure      409  {object} ErrorResponse
// @Router       /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid JSON body"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "email and password are required"})
		return
	}
	if !emailRegex.MatchString(req.Email) {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid email format"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "password must be at least 8 characters"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "failed to hash password"})
		return
	}

	user, err := h.userRepo.Create(r.Context(), req.Email, string(hashed))
	if err != nil {
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: "conflict", Message: "email already registered"})
		return
	}

	writeJSON(w, http.StatusCreated, UserResponse{UserID: user.ID, Email: user.Email, CreatedAt: user.CreatedAt})
}

// Login godoc
// @Summary      Login
// @Description  Authenticate with email and password, receive a JWT token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body     LoginRequest true "Login payload"
// @Success      200  {object} TokenResponse
// @Failure      400  {object} ErrorResponse
// @Failure      401  {object} ErrorResponse
// @Router       /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "invalid JSON body"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad_request", Message: "email and password are required"})
		return
	}

	user, err := h.userRepo.FindByEmail(r.Context(), req.Email)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "invalid email or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "invalid email or password"})
		return
	}

	token, expiresAt, err := auth.GenerateToken(user.ID, user.Email, h.jwtSecret, h.jwtExpiryHours)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "failed to generate token"})
		return
	}

	writeJSON(w, http.StatusOK, TokenResponse{Token: token, ExpiresAt: expiresAt})
}

// Me godoc
// @Summary      Get current user profile
// @Description  Returns the profile of the currently authenticated user
// @Tags         auth
// @Produce      json
// @Success      200 {object} UserResponse
// @Failure      401 {object} ErrorResponse
// @Security     BearerAuth
// @Router       /auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "missing or invalid token"})
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "user not found"})
		return
	}

	writeJSON(w, http.StatusOK, UserResponse{UserID: user.ID, Email: user.Email, CreatedAt: user.CreatedAt})
}
