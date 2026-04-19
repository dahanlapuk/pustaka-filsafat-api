package handlers

import (
	"database/sql"
	"fmt"
	"time"

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

	if err := EnsureBookStockSchema(DB); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	tx, err := DB.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal memulai transaksi"})
	}

	var bookQty int
	var bookPosisiID *int
	err = tx.QueryRow(`SELECT qty, posisi_id FROM books WHERE id = $1`, input.BookID).Scan(&bookQty, &bookPosisiID)
	if err == sql.ErrNoRows {
		_ = tx.Rollback()
		return c.Status(404).JSON(fiber.Map{"error": "Book not found"})
	}
	if err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := syncBookStockWithTotalTx(tx, input.BookID, bookPosisiID, bookQty); err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": "Gagal menyelaraskan data stok"})
	}

	var activeLoanCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM loans WHERE book_id = $1 AND tanggal_kembali IS NULL", input.BookID).Scan(&activeLoanCount)
	if err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if activeLoanCount >= bookQty {
		_ = tx.Rollback()
		return c.Status(400).JSON(fiber.Map{"error": "Stok buku sedang habis dipinjam"})
	}

	allocatedPosisiID, err := allocateLoanFromLargestStockTx(tx, input.BookID)
	if err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if allocatedPosisiID == nil && bookQty > 1 {
		_ = tx.Rollback()
		return c.Status(400).JSON(fiber.Map{"error": "Distribusi stok tidak tersedia untuk alokasi pinjaman"})
	}

	query := `
		INSERT INTO loans (book_id, nama_peminjam, catatan)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	var id int
	err = tx.QueryRow(query, input.BookID, input.NamaPeminjam, input.Catatan).Scan(&id)
	if err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if _, err := tx.Exec(`
		INSERT INTO loan_stock_allocations (loan_id, book_id, posisi_id, qty)
		VALUES ($1, $2, $3, 1)
	`, id, input.BookID, allocatedPosisiID); err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": "Gagal menyimpan alokasi pinjaman"})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	adminID, adminNama := getLoanActor(c)
	entityName := input.NamaPeminjam
	_ = LogActivity(DB, adminID, adminNama, "LOAN_CREATE", models.EntityLoan, &id, &entityName, map[string]interface{}{
		"book_id":             input.BookID,
		"nama_peminjam":       input.NamaPeminjam,
		"allocated_posisi_id": allocatedPosisiID,
	})

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

	if err := EnsureBookStockSchema(DB); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	tx, err := DB.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal memulai transaksi"})
	}

	// Update return date
	_, err = tx.Exec("UPDATE loans SET tanggal_kembali = NOW() WHERE id = $1", id)
	if err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	_, err = tx.Exec(`
		UPDATE loan_stock_allocations
		SET returned_at = NOW()
		WHERE loan_id = $1 AND returned_at IS NULL
	`, id)
	if err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	adminID, adminNama := getLoanActor(c)
	entityName := fmt.Sprintf("Loan #%s", id)
	_ = LogActivity(DB, adminID, adminNama, "LOAN_RETURN", models.EntityLoan, &loanID, &entityName, map[string]interface{}{
		"loan_id": loanID,
	})

	return c.JSON(fiber.Map{"message": "Book returned"})
}

// GetLoanDetail - GET /api/loans/:id
func GetLoanDetail(c *fiber.Ctx) error {
	loanID := c.Params("id")

	query := `
		SELECT 
			l.id, l.book_id, l.nama_peminjam, l.tanggal_pinjam, l.tanggal_kembali, l.catatan, l.dicatat_oleh,
			b.judul as judul_buku, b.kode as kode_buku
		FROM loans l
		JOIN books b ON l.book_id = b.id
		WHERE l.id = $1
	`

	var loan models.Loan
	err := DB.QueryRow(query, loanID).Scan(
		&loan.ID, &loan.BookID, &loan.NamaPeminjam, &loan.TanggalPinjam, &loan.TanggalKembali,
		&loan.Catatan, &loan.DicatatOleh, &loan.JudulBuku, &loan.KodeBuku,
	)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Loan not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Get allocation detail
	allocRows, err := DB.Query(`
		SELECT lsa.id, lsa.posisi_id, p.kode, p.rak, lsa.qty, lsa.allocated_at, lsa.returned_at
		FROM loan_stock_allocations lsa
		LEFT JOIN posisi p ON lsa.posisi_id = p.id
		WHERE lsa.loan_id = $1
		ORDER BY lsa.allocated_at DESC
	`, loanID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer allocRows.Close()

	type AllocationDetail struct {
		AllocationID *int       `json:"allocation_id"`
		PosisiID     *int       `json:"posisi_id,omitempty"`
		PosisiKode   *string    `json:"posisi_kode,omitempty"`
		PosisiRak    *string    `json:"posisi_rak,omitempty"`
		Qty          int        `json:"qty"`
		AllocatedAt  time.Time  `json:"allocated_at"`
		ReturnedAt   *time.Time `json:"returned_at,omitempty"`
	}

	allocations := []AllocationDetail{}
	for allocRows.Next() {
		var alloc AllocationDetail
		var allocationID int
		if err := allocRows.Scan(&allocationID, &alloc.PosisiID, &alloc.PosisiKode, &alloc.PosisiRak, &alloc.Qty, &alloc.AllocatedAt, &alloc.ReturnedAt); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		alloc.AllocationID = &allocationID
		allocations = append(allocations, alloc)
	}

	return c.JSON(fiber.Map{
		"id":              loan.ID,
		"book_id":         loan.BookID,
		"judul_buku":      loan.JudulBuku,
		"kode_buku":       loan.KodeBuku,
		"nama_peminjam":   loan.NamaPeminjam,
		"tanggal_pinjam":  loan.TanggalPinjam,
		"tanggal_kembali": loan.TanggalKembali,
		"catatan":         loan.Catatan,
		"dicatat_oleh":    loan.DicatatOleh,
		"allocations":     allocations,
	})
}

// GetBookStockAvailability - GET /api/books/:id/stock-availability
func GetBookStockAvailability(c *fiber.Ctx) error {
	bookID := c.Params("id")

	if err := EnsureBookStockSchema(DB); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Get stock locations with available qty
	query := `
		SELECT 
			s.posisi_id,
			p.kode as posisi_kode,
			p.rak as posisi_rak,
			s.qty,
			COALESCE(SUM(CASE WHEN a.returned_at IS NULL THEN a.qty ELSE 0 END), 0) as borrowed,
			(s.qty - COALESCE(SUM(CASE WHEN a.returned_at IS NULL THEN a.qty ELSE 0 END), 0)) as available
		FROM book_stock_locations s
		LEFT JOIN posisi p ON s.posisi_id = p.id
		LEFT JOIN loan_stock_allocations a ON a.book_id = s.book_id AND a.posisi_id IS NOT DISTINCT FROM s.posisi_id
		WHERE s.book_id = $1
		GROUP BY s.posisi_id, p.kode, p.rak, s.qty
		ORDER BY (s.qty - COALESCE(SUM(CASE WHEN a.returned_at IS NULL THEN a.qty ELSE 0 END), 0)) DESC, s.qty DESC
	`

	rows, err := DB.Query(query, bookID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type LocationAvailability struct {
		PosisiID     *int    `json:"posisi_id,omitempty"`
		PosisiKode   *string `json:"posisi_kode,omitempty"`
		PosisiRak    *string `json:"posisi_rak,omitempty"`
		TotalQty     int     `json:"total_qty"`
		BorrowedQty  int     `json:"borrowed_qty"`
		AvailableQty int     `json:"available_qty"`
	}

	locations := []LocationAvailability{}
	totalQty := 0
	totalBorrowed := 0

	for rows.Next() {
		var loc LocationAvailability
		if err := rows.Scan(&loc.PosisiID, &loc.PosisiKode, &loc.PosisiRak, &loc.TotalQty, &loc.BorrowedQty, &loc.AvailableQty); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		locations = append(locations, loc)
		totalQty += loc.TotalQty
		totalBorrowed += loc.BorrowedQty
	}

	return c.JSON(fiber.Map{
		"book_id":       bookID,
		"total_qty":     totalQty,
		"borrowed_qty":  totalBorrowed,
		"available_qty": totalQty - totalBorrowed,
		"locations":     locations,
	})
}

func getLoanActor(c *fiber.Ctx) (*int, string) {
	if raw := c.Locals("adminID"); raw != nil {
		if id, ok := raw.(int); ok {
			var nama string
			if err := DB.QueryRow("SELECT nama FROM admins WHERE id = $1", id).Scan(&nama); err == nil {
				return &id, nama
			}
			return &id, "Admin"
		}
	}
	return nil, "System"
}
