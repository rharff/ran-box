package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

const userIDKey = "user_id"
const userEmailKey = "user_email"

// Middleware returns a Fiber handler that validates JWT from the Authorization header.
// On success, it injects user_id and user_email into ctx.Locals.
func Middleware(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "unauthorized",
				"message": "missing Authorization header",
			})
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "unauthorized",
				"message": "invalid Authorization format, expected: Bearer <token>",
			})
		}

		claims, err := ParseToken(parts[1], jwtSecret)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   "unauthorized",
				"message": err.Error(),
			})
		}

		c.Locals(userIDKey, claims.UserID)
		c.Locals(userEmailKey, claims.Email)
		return c.Next()
	}
}

// GetUserID extracts the authenticated user ID from Fiber context locals.
func GetUserID(c *fiber.Ctx) (int64, bool) {
	id, ok := c.Locals(userIDKey).(int64)
	return id, ok
}
