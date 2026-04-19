package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"pustaka-filsafat/models"
	"strconv"
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

func getSessionAdmin(db *sql.DB, c *fiber.Ctx) (*models.Admin, error) {
	adminIDVal := c.Locals("adminID")
	adminID, ok := adminIDVal.(int)
	if !ok || adminID == 0 {
		return nil, errors.New("Session admin tidak valid")
	}

	var admin models.Admin
	err := db.QueryRow(`
		SELECT id, nama, nickname, email, role, title, is_superadmin, password_changed_at, created_at
		FROM admins WHERE id = $1
	`, adminID).Scan(&admin.ID, &admin.Nama, &admin.Nickname, &admin.Email, &admin.Role, &admin.Title, &admin.IsSuperadmin, &admin.PasswordChangedAt, &admin.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &admin, nil
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
		if err := EnsureAuthSessionSchema(db); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Sistem sesi belum siap. Silakan coba lagi."})
		}

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
			if logErr := LogActivity(db, &admin.ID, admin.Nama, "LOGIN_FAILED", models.EntityAdmin, &admin.ID, &admin.Nama, nil); logErr != nil {
				log.Printf("[LoginAdmin] gagal mencatat LOGIN_FAILED: %v", logErr)
			}
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
		if logErr := LogActivity(db, &admin.ID, admin.Nama, models.ActionLogin, models.EntityAdmin, &admin.ID, &admin.Nama, nil); logErr != nil {
			log.Printf("[LoginAdmin] gagal mencatat LOGIN: %v", logErr)
		}

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

// ResetPasswordBySuper - reset admin password (superadmin only, untuk recover password yang corrupt akibat mismatch encoding)
func ResetPasswordBySuper(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		admin, err := getSessionAdmin(db, c)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Session admin tidak valid"})
		}
		if !admin.IsSuperadmin {
			return c.Status(403).JSON(fiber.Map{"error": "Hanya superadmin yang bisa reset password"})
		}

		targetID := c.Params("id")
		if targetID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "ID admin diperlukan"})
		}

		var input struct {
			NewPassword string `json:"new_password"`
		}
		if err := c.BodyParser(&input); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
		}

		if input.NewPassword == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Password baru diperlukan"})
		}

		decodedPassword, _ := decodePassword(input.NewPassword)
		if len(decodedPassword) < 6 {
			return c.Status(400).JSON(fiber.Map{"error": "Password minimal 6 karakter"})
		}

		// Get target admin info
		var targetNama string
		var targetIsSuper bool
		err = db.QueryRow(`SELECT nama, is_superadmin FROM admins WHERE id = $1`, targetID).Scan(&targetNama, &targetIsSuper)
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "Admin tidak ditemukan"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data admin"})
		}

		// Prevent changing own password dari endpoint ini (use /admins/:id/password instead)
		if targetID == fmt.Sprintf("%d", admin.ID) {
			return c.Status(400).JSON(fiber.Map{"error": "Gunakan endpoint change password untuk mengubah password sendiri"})
		}

		// Hash new password
		newHash, err := bcrypt.GenerateFromPassword([]byte(decodedPassword), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal memproses password"})
		}

		// Update password
		_, err = db.Exec(`
			UPDATE admins SET password_hash = $1, password_changed_at = NOW() WHERE id = $2
		`, string(newHash), targetID)

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal reset password"})
		}

		// Log activity
		targetIDInt, _ := strconv.Atoi(targetID)
		LogActivity(db, &admin.ID, admin.Nama, "RESET_PASSWORD", models.EntityAdmin, &targetIDInt, &targetNama, fiber.Map{
			"reset_by": admin.Nama,
		})

		return c.JSON(fiber.Map{
			"message":    "Password admin berhasil direset",
			"admin_id":   targetIDInt,
			"admin_nama": targetNama,
		})
	}
}

// CreateAdmin - create new admin (superadmin only)
func CreateAdmin(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		admin, err := getSessionAdmin(db, c)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Session admin tidak valid"})
		}
		if !admin.IsSuperadmin {
			return c.Status(403).JSON(fiber.Map{"error": "Hanya superadmin yang bisa menambah admin"})
		}

		var input struct {
			Nama      string `json:"nama"`
			Nickname  string `json:"nickname"`
			Email     string `json:"email"`
			Title     string `json:"title"`
			Password  string `json:"password"`
			IsSuper   bool   `json:"is_superadmin"`
			CreatedBy int    `json:"created_by_id"`
		}
		if err := c.BodyParser(&input); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
		}
		if input.Nama == "" || input.Password == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Nama dan password wajib diisi"})
		}
		if input.Email == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Email wajib diisi"})
		}

		decodedPassword, _ := decodePassword(input.Password)
		if len(decodedPassword) < 6 {
			return c.Status(400).JSON(fiber.Map{"error": "Password minimal 6 karakter"})
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(decodedPassword), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal memproses password"})
		}

		var exists bool
		_ = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM admins WHERE LOWER(nama) = LOWER($1))`, input.Nama).Scan(&exists)
		if exists {
			return c.Status(409).JSON(fiber.Map{"error": "Nama admin sudah dipakai"})
		}

		var created models.Admin
		err = db.QueryRow(`
			INSERT INTO admins (nama, nickname, email, title, password_hash, role, is_superadmin, password_changed_at)
			VALUES ($1, NULLIF($2, ''), $3, NULLIF($4, ''), $5, 'admin', $6, NOW())
			RETURNING id, nama, nickname, email, role, title, is_superadmin, password_changed_at, created_at
		`, input.Nama, input.Nickname, input.Email, input.Title, string(passwordHash), input.IsSuper).Scan(
			&created.ID, &created.Nama, &created.Nickname, &created.Email, &created.Role, &created.Title, &created.IsSuperadmin, &created.PasswordChangedAt, &created.CreatedAt,
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal membuat admin"})
		}

		LogActivity(db, &admin.ID, admin.Nama, "CREATE_ADMIN", models.EntityAdmin, &created.ID, &created.Nama, fiber.Map{
			"created_by": admin.Nama,
		})

		return c.Status(201).JSON(fiber.Map{
			"message": "Admin berhasil dibuat",
			"admin":   created,
		})
	}
}

// UpdateAdminBySuper - update admin data (superadmin only)
func UpdateAdminBySuper(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		admin, err := getSessionAdmin(db, c)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Session admin tidak valid"})
		}
		if !admin.IsSuperadmin {
			return c.Status(403).JSON(fiber.Map{"error": "Hanya superadmin yang bisa mengubah admin"})
		}

		targetID := c.Params("id")
		var input struct {
			Nama     string `json:"nama"`
			Nickname string `json:"nickname"`
			Email    string `json:"email"`
			Title    string `json:"title"`
			IsSuper  bool   `json:"is_superadmin"`
		}
		if err := c.BodyParser(&input); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
		}
		if input.Nama == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Nama wajib diisi"})
		}

		var old models.Admin
		err = db.QueryRow(`
			SELECT id, nama, nickname, email, role, title, is_superadmin, password_changed_at, created_at
			FROM admins WHERE id = $1
		`, targetID).Scan(&old.ID, &old.Nama, &old.Nickname, &old.Email, &old.Role, &old.Title, &old.IsSuperadmin, &old.PasswordChangedAt, &old.CreatedAt)
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "Admin tidak ditemukan"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data admin"})
		}

		_, err = db.Exec(`
			UPDATE admins
			SET nama = $1, nickname = NULLIF($2, ''), email = $3, title = NULLIF($4, ''), is_superadmin = $5
			WHERE id = $6
		`, input.Nama, input.Nickname, input.Email, input.Title, input.IsSuper, targetID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengubah admin"})
		}

		var updated models.Admin
		err = db.QueryRow(`
			SELECT id, nama, nickname, email, role, title, is_superadmin, password_changed_at, created_at
			FROM admins WHERE id = $1
		`, targetID).Scan(&updated.ID, &updated.Nama, &updated.Nickname, &updated.Email, &updated.Role, &updated.Title, &updated.IsSuperadmin, &updated.PasswordChangedAt, &updated.CreatedAt)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data admin"})
		}

		action := "UPDATE_ADMIN"
		if old.IsSuperadmin != updated.IsSuperadmin {
			if updated.IsSuperadmin {
				action = "PROMOTE_ADMIN"
			} else {
				action = "DEMOTE_ADMIN"
			}
		}

		LogActivity(db, &admin.ID, admin.Nama, action, models.EntityAdmin, &updated.ID, &updated.Nama, fiber.Map{
			"old_name":          old.Nama,
			"new_name":          updated.Nama,
			"old_nickname":      old.Nickname,
			"new_nickname":      updated.Nickname,
			"old_title":         old.Title,
			"new_title":         updated.Title,
			"old_is_superadmin": old.IsSuperadmin,
			"new_is_superadmin": updated.IsSuperadmin,
		})

		return c.JSON(fiber.Map{
			"message": "Admin berhasil diperbarui",
			"admin":   updated,
		})
	}
}

// DeleteAdmin - delete admin (superadmin only)
func DeleteAdmin(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		admin, err := getSessionAdmin(db, c)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Session admin tidak valid"})
		}
		if !admin.IsSuperadmin {
			return c.Status(403).JSON(fiber.Map{"error": "Hanya superadmin yang bisa menghapus admin"})
		}

		targetID := c.Params("id")
		if targetID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "ID admin diperlukan"})
		}
		if targetID == fmt.Sprintf("%d", admin.ID) {
			return c.Status(400).JSON(fiber.Map{"error": "Tidak bisa menghapus akun sendiri"})
		}

		var target models.Admin
		err = db.QueryRow(`
			SELECT id, nama, nickname, email, role, title, is_superadmin, password_changed_at, created_at
			FROM admins WHERE id = $1
		`, targetID).Scan(&target.ID, &target.Nama, &target.Nickname, &target.Email, &target.Role, &target.Title, &target.IsSuperadmin, &target.PasswordChangedAt, &target.CreatedAt)
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "Admin tidak ditemukan"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data admin"})
		}
		if target.IsSuperadmin {
			return c.Status(403).JSON(fiber.Map{"error": "Tidak bisa menghapus superadmin"})
		}

		_, err = db.Exec(`DELETE FROM admins WHERE id = $1`, targetID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal menghapus admin"})
		}

		LogActivity(db, &admin.ID, admin.Nama, "DELETE_ADMIN", models.EntityAdmin, &target.ID, &target.Nama, nil)

		return c.JSON(fiber.Map{
			"message": "Admin berhasil dihapus",
			"admin":   fiber.Map{"id": target.ID, "nama": target.Nama},
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
		if logErr := LogActivity(db, &adminID, adminNama, models.ActionLogout, models.EntityAdmin, &adminID, &entityName, nil); logErr != nil {
			log.Printf("[LogoutAdmin] gagal mencatat LOGOUT: %v", logErr)
		}

		return c.JSON(fiber.Map{
			"message": "Logout berhasil",
		})
	}
}
