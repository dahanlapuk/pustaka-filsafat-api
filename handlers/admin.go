package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"pustaka-filsafat/models"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// Password expiry duration - 6 months
const PasswordExpiryDuration = 6 * 30 * 24 * time.Hour // ~6 months

// Session expiry duration - 10 hours
const SessionExpiryDuration = 10 * time.Hour

func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashSessionToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// decodePassword - decode Base64 encoded password from frontend
func decodePassword(encoded string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return encoded, nil // Return as-is if not base64 (backward compatibility)
	}
	return string(decoded), nil
}

// GetAdmins - list semua admin (untuk login selection)
func GetAdmins(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(`
			SELECT id, nama, nickname, email, role, title, is_superadmin, created_at
			FROM admins ORDER BY id
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data admin"})
		}
		defer rows.Close()

		var admins []models.Admin
		for rows.Next() {
			var admin models.Admin
			err := rows.Scan(&admin.ID, &admin.Nama, &admin.Nickname, &admin.Email, &admin.Role, &admin.Title, &admin.IsSuperadmin, &admin.CreatedAt)
			if err != nil {
				continue
			}
			admins = append(admins, admin)
		}

		if admins == nil {
			admins = []models.Admin{}
		}

		return c.JSON(admins)
	}
}

// GetCurrentAdmin - get admin by ID
func GetCurrentAdmin(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		adminID := c.QueryInt("admin_id", 0)
		if adminID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "admin_id diperlukan"})
		}

		var admin models.Admin
		err := db.QueryRow(`
			SELECT id, nama, nickname, email, role, title, is_superadmin, password_changed_at, created_at
			FROM admins WHERE id = $1
		`, adminID).Scan(&admin.ID, &admin.Nama, &admin.Nickname, &admin.Email, &admin.Role, &admin.Title, &admin.IsSuperadmin, &admin.PasswordChangedAt, &admin.CreatedAt)

		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "Admin tidak ditemukan"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data admin"})
		}

		// Check password expiry
		passwordExpired := false
		if admin.PasswordChangedAt != nil {
			if time.Since(*admin.PasswordChangedAt) > PasswordExpiryDuration {
				passwordExpired = true
			}
		}

		return c.JSON(fiber.Map{
			"id":                  admin.ID,
			"nama":                admin.Nama,
			"nickname":            admin.Nickname,
			"email":               admin.Email,
			"role":                admin.Role,
			"title":               admin.Title,
			"is_superadmin":       admin.IsSuperadmin,
			"password_changed_at": admin.PasswordChangedAt,
			"password_expired":    passwordExpired,
			"created_at":          admin.CreatedAt,
		})
	}
}

// LoginAdmin - login with password
func LoginAdmin(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input models.AdminLoginInput
		if err := c.BodyParser(&input); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
		}

		if input.AdminID == 0 || input.Password == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Admin ID dan password diperlukan"})
		}

		var admin models.Admin
		var passwordHash string
		err := db.QueryRow(`
			SELECT id, nama, nickname, email, password_hash, role, title, is_superadmin, password_changed_at, created_at
			FROM admins WHERE id = $1
		`, input.AdminID).Scan(&admin.ID, &admin.Nama, &admin.Nickname, &admin.Email, &passwordHash, &admin.Role, &admin.Title, &admin.IsSuperadmin, &admin.PasswordChangedAt, &admin.CreatedAt)

		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "Admin tidak ditemukan"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data admin"})
		}

		// Decode Base64 password
		decodedPassword, _ := decodePassword(input.Password)

		// Verify password
		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(decodedPassword)); err != nil {
			// Log failed attempt
			LogActivity(db, &admin.ID, admin.Nama, "LOGIN_FAILED", models.EntityAdmin, &admin.ID, &admin.Nama, nil)
			return c.Status(401).JSON(fiber.Map{"error": "Password salah"})
		}

		// Check password expiry
		passwordExpired := false
		if admin.PasswordChangedAt != nil {
			if time.Since(*admin.PasswordChangedAt) > PasswordExpiryDuration {
				passwordExpired = true
			}
		}

		// Log successful login
		LogActivity(db, &admin.ID, admin.Nama, models.ActionLogin, models.EntityAdmin, &admin.ID, &admin.Nama, nil)

		issuedAt := time.Now().UTC()
		expiresAt := issuedAt.Add(SessionExpiryDuration)

		token, err := generateSessionToken()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal membuat sesi login"})
		}
		tokenHash := hashSessionToken(token)

		// Single-session policy: login baru menonaktifkan sesi aktif sebelumnya.
		_, err = db.Exec(`
			UPDATE admin_sessions
			SET invalidated_at = NOW()
			WHERE admin_id = $1 AND invalidated_at IS NULL
		`, admin.ID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal memperbarui sesi aktif"})
		}

		_, err = db.Exec(`
			INSERT INTO admin_sessions (admin_id, token_hash, issued_at, expires_at)
			VALUES ($1, $2, $3, $4)
		`, admin.ID, tokenHash, issuedAt, expiresAt)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal menyimpan sesi login"})
		}

		return c.JSON(fiber.Map{
			"message":            "Login berhasil",
			"session_token":      token,
			"session_started_at": issuedAt,
			"session_expires_at": expiresAt,
			"admin": fiber.Map{
				"id":                  admin.ID,
				"nama":                admin.Nama,
				"nickname":            admin.Nickname,
				"email":               admin.Email,
				"role":                admin.Role,
				"title":               admin.Title,
				"is_superadmin":       admin.IsSuperadmin,
				"password_changed_at": admin.PasswordChangedAt,
				"password_expired":    passwordExpired,
				"created_at":          admin.CreatedAt,
			},
		})
	}
}

// UpdateProfile - update admin profile (nama, nickname, email)
func UpdateProfile(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		adminID := c.Params("id")

		var input models.AdminProfileUpdate
		if err := c.BodyParser(&input); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
		}

		// Get current admin data for logging
		var oldNama string
		err := db.QueryRow(`SELECT nama FROM admins WHERE id = $1`, adminID).Scan(&oldNama)
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "Admin tidak ditemukan"})
		}

		// Update profile
		_, err = db.Exec(`
			UPDATE admins SET nama = $1, nickname = $2, email = $3 WHERE id = $4
		`, input.Nama, input.Nickname, input.Email, adminID)

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengupdate profil"})
		}

		// Get updated admin
		var admin models.Admin
		err = db.QueryRow(`
			SELECT id, nama, nickname, email, role, title, is_superadmin, password_changed_at, created_at
			FROM admins WHERE id = $1
		`, adminID).Scan(&admin.ID, &admin.Nama, &admin.Nickname, &admin.Email, &admin.Role, &admin.Title, &admin.IsSuperadmin, &admin.PasswordChangedAt, &admin.CreatedAt)

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data admin"})
		}

		// Log activity
		LogActivity(db, &admin.ID, admin.Nama, models.ActionUpdate, models.EntityAdmin, &admin.ID, &admin.Nama, nil)

		return c.JSON(fiber.Map{
			"message": "Profil berhasil diperbarui",
			"admin":   admin,
		})
	}
}

// ChangePassword - change admin password
func ChangePassword(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		adminID := c.Params("id")

		var input models.AdminPasswordChange
		if err := c.BodyParser(&input); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
		}

		if input.OldPassword == "" || input.NewPassword == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Password lama dan baru diperlukan"})
		}

		// Decode Base64 passwords
		decodedOldPassword, _ := decodePassword(input.OldPassword)
		decodedNewPassword, _ := decodePassword(input.NewPassword)

		if len(decodedNewPassword) < 6 {
			return c.Status(400).JSON(fiber.Map{"error": "Password baru minimal 6 karakter"})
		}

		// Get current password hash
		var currentHash string
		var adminNama string
		var adminIDInt int
		err := db.QueryRow(`SELECT id, nama, password_hash FROM admins WHERE id = $1`, adminID).Scan(&adminIDInt, &adminNama, &currentHash)
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "Admin tidak ditemukan"})
		}

		// Verify old password
		if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(decodedOldPassword)); err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Password lama salah"})
		}

		// Hash new password
		newHash, err := bcrypt.GenerateFromPassword([]byte(decodedNewPassword), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal memproses password"})
		}

		// Update password
		_, err = db.Exec(`
			UPDATE admins SET password_hash = $1, password_changed_at = NOW() WHERE id = $2
		`, string(newHash), adminID)

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengubah password"})
		}

		// Log activity
		LogActivity(db, &adminIDInt, adminNama, "CHANGE_PASSWORD", models.EntityAdmin, &adminIDInt, &adminNama, nil)

		return c.JSON(fiber.Map{
			"message": "Password berhasil diubah",
		})
	}
}

// LogoutAdmin - logout and log activity
func LogoutAdmin(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionToken := c.Get("X-Session-Token")
		if sessionToken == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Session token diperlukan"})
		}

		var input struct {
			AdminID int    `json:"admin_id"`
			Nama    string `json:"nama"`
		}
		if len(c.Body()) > 0 {
			if err := c.BodyParser(&input); err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "Input logout tidak valid"})
			}
		}

		tokenHash := hashSessionToken(sessionToken)
		result, err := db.Exec(`
			UPDATE admin_sessions
			SET invalidated_at = NOW()
			WHERE token_hash = $1 AND invalidated_at IS NULL
		`, tokenHash)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal logout"})
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return c.Status(401).JSON(fiber.Map{"error": "Session tidak valid atau sudah logout"})
		}

		adminIDVal := c.Locals("adminID")
		adminID, ok := adminIDVal.(int)
		if !ok || adminID == 0 {
			return c.Status(401).JSON(fiber.Map{"error": "Session admin tidak valid"})
		}

		adminNama := input.Nama
		if adminNama == "" {
			_ = db.QueryRow(`SELECT nama FROM admins WHERE id = $1`, adminID).Scan(&adminNama)
		}

		// Log logout
		entityName := fmt.Sprintf("admin-%d", adminID)
		if adminNama != "" {
			entityName = adminNama
		}
		LogActivity(db, &adminID, adminNama, models.ActionLogout, models.EntityAdmin, &adminID, &entityName, nil)

		return c.JSON(fiber.Map{
			"message": "Logout berhasil",
		})
	}
}
