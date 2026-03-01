package handlers

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Feature flag - controlled by environment variable
var GenerateCodeEnabled = os.Getenv("ENABLE_GENERATE_CODE") == "true"

// GenerateBookCode - POST /api/books/:id/generate-code
// Generate kode buku berdasarkan kategori dan urutan
// DISABLED by default - requires ENABLE_GENERATE_CODE=true
func GenerateBookCode(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if feature is enabled
		if !GenerateCodeEnabled {
			return c.Status(403).JSON(fiber.Map{
				"error":   "Fitur generate kode dinonaktifkan",
				"message": "Fitur ini memerlukan persetujuan kantor. Hubungi administrator.",
			})
		}

		bookID := c.Params("id")

		// Get book data
		var book struct {
			ID         int
			Judul      string
			KategoriID *int
			Kode       *string
		}

		err := db.QueryRow(`
			SELECT id, judul, kategori_id, kode FROM books WHERE id = $1
		`, bookID).Scan(&book.ID, &book.Judul, &book.KategoriID, &book.Kode)

		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "Buku tidak ditemukan"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data buku"})
		}

		// Check if already has code
		if book.Kode != nil && *book.Kode != "" {
			return c.Status(400).JSON(fiber.Map{
				"error":   "Buku sudah memiliki kode",
				"kode":    *book.Kode,
				"message": "Gunakan fitur edit untuk mengubah kode",
			})
		}

		// Generate code based on category
		var prefix string
		if book.KategoriID != nil {
			var catName string
			db.QueryRow("SELECT nama FROM categories WHERE id = $1", *book.KategoriID).Scan(&catName)
			// Take first 3 letters of category
			if len(catName) >= 3 {
				prefix = catName[:3]
			} else {
				prefix = catName
			}
		} else {
			prefix = "UNK" // Unknown category
		}

		// Count books in same category for sequence number
		var count int
		if book.KategoriID != nil {
			db.QueryRow(`
				SELECT COUNT(*) FROM books WHERE kategori_id = $1 AND kode IS NOT NULL
			`, *book.KategoriID).Scan(&count)
		} else {
			db.QueryRow("SELECT COUNT(*) FROM books WHERE kode IS NOT NULL").Scan(&count)
		}

		// Generate code: PREFIX-YYMMDD-SEQ
		now := time.Now()
		kode := fmt.Sprintf("%s-%s-%04d", prefix, now.Format("060102"), count+1)

		// Update book with new code
		_, err = db.Exec("UPDATE books SET kode = $1, updated_at = NOW() WHERE id = $2", kode, bookID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal menyimpan kode"})
		}

		return c.JSON(fiber.Map{
			"message": "Kode berhasil di-generate",
			"kode":    kode,
			"book_id": book.ID,
		})
	}
}

// GetBooksWithoutCode - GET /api/books/no-code
// Get list of books without code
func GetBooksWithoutCode(db *sql.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rows, err := db.Query(`
			SELECT b.id, b.judul, b.kategori_id, c.nama as kategori_nama, b.created_at
			FROM books b
			LEFT JOIN categories c ON b.kategori_id = c.id
			WHERE b.kode IS NULL OR b.kode = ''
			ORDER BY b.created_at DESC
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data"})
		}
		defer rows.Close()

		var books []map[string]interface{}
		for rows.Next() {
			var id int
			var judul string
			var kategoriID *int
			var kategoriNama *string
			var createdAt time.Time

			rows.Scan(&id, &judul, &kategoriID, &kategoriNama, &createdAt)

			books = append(books, map[string]interface{}{
				"id":            id,
				"judul":         judul,
				"kategori_id":   kategoriID,
				"kategori_nama": kategoriNama,
				"created_at":    createdAt,
			})
		}

		if books == nil {
			books = []map[string]interface{}{}
		}

		return c.JSON(fiber.Map{
			"count":           len(books),
			"books":           books,
			"feature_enabled": GenerateCodeEnabled,
		})
	}
}
