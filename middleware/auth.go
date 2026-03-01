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
		// Get admin ID from header
		adminIDStr := c.Get("X-Admin-ID")
		sessionStart := c.Get("X-Session-Start")

		// Skip auth for public routes
		path := c.Path()
		if isPublicRoute(path) {
			return c.Next()
		}

		// Check if headers exist
		if adminIDStr == "" {
			return c.Status(401).JSON(fiber.Map{
				"error": "Unauthorized - silakan login terlebih dahulu",
			})
		}

		// Validate admin ID
		adminID, err := strconv.Atoi(adminIDStr)
		if err != nil || adminID <= 0 {
			return c.Status(401).JSON(fiber.Map{
				"error": "Invalid admin session",
			})
		}

		// Validate session time
		if sessionStart != "" {
			sessionTime, err := strconv.ParseInt(sessionStart, 10, 64)
			if err == nil {
				elapsed := time.Now().UnixMilli() - sessionTime
				if elapsed > SessionDuration.Milliseconds() {
					return c.Status(401).JSON(fiber.Map{
						"error": "Session expired - silakan login kembali",
					})
				}
			}
		}

		// Verify admin exists in database
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM admins WHERE id = $1)", adminID).Scan(&exists)
		if err != nil || !exists {
			return c.Status(401).JSON(fiber.Map{
				"error": "Admin tidak ditemukan",
			})
		}

		// Store admin ID in context for handlers to use
		c.Locals("adminID", adminID)

		return c.Next()
	}
}

// isPublicRoute checks if route should skip auth
func isPublicRoute(path string) bool {
	publicPaths := []string{
		"/health",
		"/api/books",      // GET only - read public catalog
		"/api/categories", // GET only - read categories
		"/api/posisi",     // GET only - read positions
		"/api/admins",     // GET only - list admins for login
	}

	for _, p := range publicPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}

	return false
}
