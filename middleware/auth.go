package middleware

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Session duration - 10 hours
const SessionDuration = 10 * time.Hour

// AdminAuth middleware - validates admin session from header
func AdminAuth(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// ===== PUBLIC ROUTE CHECK (FIRST) =====
		if isPublicRoute(c) {
			return c.Next()
		}

		// ===== GET HEADERS =====
		adminIDStr := c.Get("X-Admin-ID")
		sessionStart := c.Get("X-Session-Start")

		// ===== HEADER REQUIRED =====
		if adminIDStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - silakan login terlebih dahulu",
			})
		}

		// ===== VALIDATE ADMIN ID =====
		adminID, err := strconv.Atoi(adminIDStr)
		if err != nil || adminID <= 0 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid admin session",
			})
		}

		// ===== VALIDATE SESSION TIME =====
		if sessionStart != "" {
			sessionTime, err := strconv.ParseInt(sessionStart, 10, 64)
			if err == nil {
				elapsed := time.Now().UnixMilli() - sessionTime
				if elapsed > SessionDuration.Milliseconds() {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "Session expired - silakan login kembali",
					})
				}
			}
		}

		// ===== VERIFY ADMIN EXISTS =====
		var exists bool
		err = db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM admins WHERE id = $1)",
			adminID,
		).Scan(&exists)

		if err != nil || !exists {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Admin tidak ditemukan",
			})
		}

		// ===== STORE IN CONTEXT =====
		c.Locals("adminID", adminID)

		return c.Next()
	}
}

// isPublicRoute determines which endpoints are public
func isPublicRoute(c *fiber.Ctx) bool {
	path := c.Path()
	method := c.Method()

	// --- health always public
	if path == "/health" {
		return true
	}

	// --- PUBLIC GET endpoints (catalog access)
	if method == fiber.MethodGet {
		publicGetPrefixes := []string{
			"/api/books",
			"/api/categories",
			"/api/posisi",
			"/api/admins",
		}

		for _, p := range publicGetPrefixes {
			if strings.HasPrefix(path, p) {
				return true
			}
		}
	}

	return false
}