package handlers

import (
	"database/sql"

	"pustaka-filsafat/models"

	"github.com/gofiber/fiber/v2"
)

// GetLoans - GET /api/loans (active loans only by default)
func GetLoans(c *fiber.Ctx) error {
	showAll := c.Query("all") == "true"

	query := `
		SELECT 
			l.id, l.book_id, l.nama_peminjam, l.tanggal_pinjam, l.tanggal_kembali, l.catatan, l.dicatat_oleh,
			b.judul as judul_buku, b.kode as kode_buku
		FROM loans l
		JOIN books b ON l.book_id = b.id
	`

	if !showAll {
		query += " WHERE l.tanggal_kembali IS NULL"
	}

	query += " ORDER BY l.tanggal_pinjam DESC"

	rows, err := DB.Query(query)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	loans := []models.Loan{}
	for rows.Next() {
		var loan models.Loan
		if err := rows.Scan(
			&loan.ID, &loan.BookID, &loan.NamaPeminjam, &loan.TanggalPinjam, &loan.TanggalKembali,
			&loan.Catatan, &loan.DicatatOleh, &loan.JudulBuku, &loan.KodeBuku,
		); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		loans = append(loans, loan)
	}

	return c.JSON(loans)
}

// CreateLoan - POST /api/loans
func CreateLoan(c *fiber.Ctx) error {
	var input models.LoanInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	if input.BookID == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Book ID wajib diisi"})
	}
	if input.NamaPeminjam == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Nama peminjam wajib diisi"})
	}

	// Check if book exists
	var exists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM books WHERE id = $1)", input.BookID).Scan(&exists)
	if err != nil || !exists {
		return c.Status(404).JSON(fiber.Map{"error": "Book not found"})
	}

	// Check if book is already borrowed
	var activeLoan bool
	err = DB.QueryRow("SELECT EXISTS(SELECT 1 FROM loans WHERE book_id = $1 AND tanggal_kembali IS NULL)", input.BookID).Scan(&activeLoan)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if activeLoan {
		return c.Status(400).JSON(fiber.Map{"error": "Buku sudah dipinjam"})
	}

	query := `
		INSERT INTO loans (book_id, nama_peminjam, catatan)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	var id int
	err = DB.QueryRow(query, input.BookID, input.NamaPeminjam, input.Catatan).Scan(&id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{"id": id, "message": "Loan created"})
}

// ReturnLoan - PUT /api/loans/:id/return
func ReturnLoan(c *fiber.Ctx) error {
	id := c.Params("id")

	// Check if loan exists and is active
	var loanID int
	err := DB.QueryRow("SELECT id FROM loans WHERE id = $1 AND tanggal_kembali IS NULL", id).Scan(&loanID)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Active loan not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Update return date
	_, err = DB.Exec("UPDATE loans SET tanggal_kembali = NOW() WHERE id = $1", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Book returned"})
}
