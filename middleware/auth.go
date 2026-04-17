package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Session duration - 10 hours
const SessionDuration = 10 * time.Hour

func hashSessionToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// AdminAuth middleware - validates admin session from header
func AdminAuth(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Public routes bypass auth middleware.
		if isPublicRoute(c) {
			return c.Next()
		}

		sessionToken := strings.TrimSpace(c.Get("X-Session-Token"))
		if sessionToken == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized - session token diperlukan",
			})
		}

		tokenHash := hashSessionToken(sessionToken)

		var adminID int
		var expiresAt time.Time
		err := db.QueryRow(`
			SELECT s.admin_id, s.expires_at
			FROM admin_sessions s
			JOIN admins a ON a.id = s.admin_id
			WHERE s.token_hash = $1 AND s.invalidated_at IS NULL
			LIMIT 1
		`, tokenHash).Scan(&adminID, &expiresAt)

		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid session - silakan login kembali",
			})
		}
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Gagal memvalidasi sesi",
			})
		}

		now := time.Now().UTC()
		if now.After(expiresAt.UTC()) {
			_, _ = db.Exec(`
				UPDATE admin_sessions
				SET invalidated_at = NOW()
				WHERE token_hash = $1 AND invalidated_at IS NULL
			`, tokenHash)

			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Session expired - silakan login kembali",
			})
		}

		// Store admin identity derived from session.
		c.Locals("adminID", adminID)
		c.Locals("sessionTokenHash", tokenHash)

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

	// --- PUBLIC POST endpoints (pengajuan peminjaman)
	if method == fiber.MethodPost && path == "/api/loan-requests" {
		return true
	}

	return false
}
