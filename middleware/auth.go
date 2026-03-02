package middleware

import (
	"database/sql"
	"fmt"
	"log"
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
		fmt.Println("AUTH PATH:", c.Path())
		fmt.Println("IS PUBLIC:", isPublicRoute(c))
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

		// Validate session time
		if sessionStart != "" {
			sessionTime, err := strconv.ParseInt(sessionStart, 10, 64)
			if err == nil {
				elapsed := time.Now().UnixMilli() - sessionTime

				log.Println("SESSION DEBUG → start:", sessionTime)
				log.Println("SESSION DEBUG → now:", time.Now().UnixMilli())
				log.Println("SESSION DEBUG → elapsed(ms):", elapsed)
				log.Println("SESSION DEBUG → limit(ms):", SessionDuration.Milliseconds())

				if elapsed > SessionDuration.Milliseconds() {
					return c.Status(401).JSON(fiber.Map{
						"error": "Session expired - silakan login kembali",
					})
				}
			} else {
				log.Println("SESSION DEBUG → parse error:", err)
			}
		} else {
			log.Println("SESSION DEBUG → sessionStart header kosong")
		}

		// ===== VERIFY ADMIN EXISTS =====
		var exists bool
		err = db.QueryRow(`
    SELECT EXISTS(
        SELECT 1 FROM users 
        WHERE id = $1
        AND role IN ('magang','kaprodi','admin')
    )
`, adminID).Scan(&exists)

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

	// --- LOGIN & ADMIN LIST (PUBLIC)
	if path == "/api/admins/login" && method == fiber.MethodPost {
		return true
	}

	if path == "/api/admins" && method == fiber.MethodGet {
		return true
	}

	// --- PUBLIC GET endpoints (catalog access)
	if method == fiber.MethodGet {
		publicGetPrefixes := []string{
			"/api/books",
			"/api/categories",
			"/api/posisi",
		}

		for _, p := range publicGetPrefixes {
			if strings.HasPrefix(path, p) {
				return true
			}
		}
	}

	return false
}
