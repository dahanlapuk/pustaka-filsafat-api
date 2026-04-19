package handlers

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
)

// ── BATCH UPDATE POSISI ───────────────────────────────────────────────────────

// BatchUpdatePosisi - PUT /api/books/batch-posisi
func BatchUpdatePosisi(c *fiber.Ctx) error {
	var input struct {
		BookIDs   []int  `json:"book_ids"`
		PosisiID  int    `json:"posisi_id"`
		AdminID   *int   `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}
	if len(input.BookIDs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Pilih minimal satu buku"})
	}
	if input.PosisiID == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Posisi tujuan wajib dipilih"})
	}

	var posisiKode string
	DB.QueryRow("SELECT kode FROM posisi WHERE id = $1", input.PosisiID).Scan(&posisiKode)

	result, err := DB.Exec(`
		UPDATE books
		SET posisi_id = $1, updated_at = NOW(), updated_by = $2, updated_by_nama = $3
		WHERE id = ANY($4)
	`, input.PosisiID, input.AdminID, input.AdminNama, pq.Array(input.BookIDs))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	updated, _ := result.RowsAffected()

	adminNama := input.AdminNama
	if adminNama == "" {
		adminNama = "Unknown"
	}
	LogActivity(DB, input.AdminID, adminNama, "POSITION_CHANGE", "book", nil, nil, map[string]interface{}{
		"book_ids":    input.BookIDs,
		"posisi_id":   input.PosisiID,
		"posisi_kode": posisiKode,
		"count":       updated,
	})

	return c.JSON(fiber.Map{"message": "Posisi berhasil diperbarui", "updated": updated})
}

// ── LOAN REQUESTS ────────────────────────────────────────────────────────────

// CreateLoanRequest - POST /api/loan-requests (publik, no auth)
func CreateLoanRequest(c *fiber.Ctx) error {
	var input struct {
		BookID      int    `json:"book_id"`
		NamaPemohon string `json:"nama_pemohon"`
		Whatsapp    string `json:"whatsapp"`
		Email       string `json:"email"`
		Keperluan   string `json:"keperluan"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}
	if input.BookID == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "ID buku wajib diisi"})
	}
	if input.NamaPemohon == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Nama pemohon wajib diisi"})
	}
	if input.Whatsapp == "" && input.Email == "" {
		return c.Status(400).JSON(fiber.Map{"error": "WhatsApp atau email wajib diisi minimal satu"})
	}

	var judul string
	var isBorrowed bool
	err := DB.QueryRow(`
		SELECT b.judul,
		       EXISTS(SELECT 1 FROM loans l WHERE l.book_id = b.id AND l.tanggal_kembali IS NULL)
		FROM books b WHERE b.id = $1
	`, input.BookID).Scan(&judul, &isBorrowed)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Buku tidak ditemukan"})
	}
	if isBorrowed {
		return c.Status(409).JSON(fiber.Map{"error": "Buku sedang dipinjam orang lain"})
	}

	var existingID int
	DB.QueryRow(`
		SELECT id FROM loan_requests
		WHERE book_id = $1 AND status = 'pending'
		  AND (whatsapp = $2 OR email = $3)
		LIMIT 1
	`, input.BookID, input.Whatsapp, input.Email).Scan(&existingID)
	if existingID > 0 {
		return c.Status(409).JSON(fiber.Map{"error": "Anda sudah mengajukan peminjaman buku ini dan masih menunggu persetujuan"})
	}

	var id int
	err = DB.QueryRow(`
		INSERT INTO loan_requests (book_id, nama_pemohon, whatsapp, email, keperluan)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, input.BookID, input.NamaPemohon,
		nullStr(input.Whatsapp), nullStr(input.Email), nullStr(input.Keperluan),
	).Scan(&id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message":         "Pengajuan berhasil! Admin akan menghubungi Anda segera.",
		"nomor_pengajuan": id,
		"judul_buku":      judul,
	})
}

// GetLoanRequests - GET /api/loan-requests (admin)
func GetLoanRequests(c *fiber.Ctx) error {
	status := c.Query("status", "")

	query := `
		SELECT lr.id, lr.book_id, b.judul, lr.nama_pemohon, lr.whatsapp, lr.email,
		       lr.keperluan, lr.status, lr.processed_by, lr.processed_at,
		       lr.catatan_admin, lr.created_at
		FROM loan_requests lr
		JOIN books b ON lr.book_id = b.id
		WHERE 1=1
	`
	args := []interface{}{}
	if status != "" {
		query += " AND lr.status = $1"
		args = append(args, status)
	}
	query += " ORDER BY lr.created_at DESC"

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type LoanRequestRow struct {
		ID           int        `json:"id"`
		BookID       int        `json:"book_id"`
		JudulBuku    string     `json:"judul_buku"`
		NamaPemohon  string     `json:"nama_pemohon"`
		Whatsapp     *string    `json:"whatsapp"`
		Email        *string    `json:"email"`
		Keperluan    *string    `json:"keperluan"`
		Status       string     `json:"status"`
		ProcessedBy  *int       `json:"processed_by"`
		ProcessedAt  *time.Time `json:"processed_at"`
		CatatanAdmin *string    `json:"catatan_admin"`
		CreatedAt    time.Time  `json:"created_at"`
	}

	result := []LoanRequestRow{}
	for rows.Next() {
		var r LoanRequestRow
		if err := rows.Scan(&r.ID, &r.BookID, &r.JudulBuku, &r.NamaPemohon,
			&r.Whatsapp, &r.Email, &r.Keperluan, &r.Status,
			&r.ProcessedBy, &r.ProcessedAt, &r.CatatanAdmin, &r.CreatedAt); err != nil {
			continue
		}
		result = append(result, r)
	}
	return c.JSON(result)
}

// ApproveLoanRequest - PUT /api/loan-requests/:id/approve (admin)
func ApproveLoanRequest(c *fiber.Ctx) error {
	id := c.Params("id")
	var input struct {
		AdminID   int    `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	c.BodyParser(&input)

	var bookID int
	var namaPemohon, status string
	err := DB.QueryRow(`
		SELECT book_id, nama_pemohon, status FROM loan_requests WHERE id = $1
	`, id).Scan(&bookID, &namaPemohon, &status)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Pengajuan tidak ditemukan"})
	}
	if status != "pending" {
		return c.Status(409).JSON(fiber.Map{"error": "Pengajuan ini sudah diproses"})
	}

	_, err = DB.Exec(`
		UPDATE loan_requests
		SET status = 'approved', processed_by = $1, processed_at = NOW()
		WHERE id = $2
	`, input.AdminID, id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	_, err = DB.Exec(`
		INSERT INTO loans (book_id, nama_peminjam, catatan)
		VALUES ($1, $2, 'Dari pengajuan online')
	`, bookID, namaPemohon)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal membuat record peminjaman: " + err.Error()})
	}

	LogActivity(DB, &input.AdminID, input.AdminNama, "APPROVE_LOAN_REQUEST", "loan_request", nil, &namaPemohon, nil)
	return c.JSON(fiber.Map{"message": "Pengajuan disetujui dan peminjaman dicatat"})
}

// RejectLoanRequest - PUT /api/loan-requests/:id/reject (admin)
func RejectLoanRequest(c *fiber.Ctx) error {
	id := c.Params("id")
	var input struct {
		AdminID      int    `json:"admin_id"`
		AdminNama    string `json:"admin_nama"`
		CatatanAdmin string `json:"catatan_admin"`
	}
	c.BodyParser(&input)

	var namaPemohon, status string
	err := DB.QueryRow("SELECT nama_pemohon, status FROM loan_requests WHERE id = $1", id).
		Scan(&namaPemohon, &status)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Pengajuan tidak ditemukan"})
	}
	if status != "pending" {
		return c.Status(409).JSON(fiber.Map{"error": "Pengajuan ini sudah diproses"})
	}

	DB.Exec(`
		UPDATE loan_requests
		SET status = 'rejected', processed_by = $1, processed_at = NOW(), catatan_admin = $2
		WHERE id = $3
	`, input.AdminID, nullStr(input.CatatanAdmin), id)

	LogActivity(DB, &input.AdminID, input.AdminNama, "REJECT_LOAN_REQUEST", "loan_request", nil, &namaPemohon, nil)
	return c.JSON(fiber.Map{"message": "Pengajuan ditolak"})
}

// ── DELETE REQUESTS ──────────────────────────────────────────────────────────

// CreateDeleteRequest - POST /api/delete-requests (admin biasa)
func CreateDeleteRequest(c *fiber.Ctx) error {
	var input struct {
		BookID          int    `json:"book_id"`
		Alasan          string `json:"alasan"`
		RequestedBy     int    `json:"requested_by"`
		RequestedByNama string `json:"requested_by_nama"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}
	if input.BookID == 0 || input.Alasan == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Book ID dan alasan wajib diisi"})
	}

	var judulSnapshot string
	err := DB.QueryRow("SELECT judul FROM books WHERE id = $1", input.BookID).Scan(&judulSnapshot)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Buku tidak ditemukan"})
	}

	var existingID int
	DB.QueryRow(`SELECT id FROM delete_requests WHERE book_id = $1 AND status = 'pending' LIMIT 1`,
		input.BookID).Scan(&existingID)
	if existingID > 0 {
		return c.Status(409).JSON(fiber.Map{"error": "Sudah ada pengajuan hapus yang menunggu untuk buku ini"})
	}

	var id int
	err = DB.QueryRow(`
		INSERT INTO delete_requests (book_id, judul_snapshot, alasan, requested_by, requested_by_nama)
		VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, input.BookID, judulSnapshot, input.Alasan, input.RequestedBy, input.RequestedByNama).Scan(&id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	LogActivity(DB, &input.RequestedBy, input.RequestedByNama, "DELETE_REQUEST", "book",
		&input.BookID, &judulSnapshot, map[string]interface{}{"alasan": input.Alasan})

	return c.Status(201).JSON(fiber.Map{"message": "Pengajuan hapus berhasil dikirim", "id": id})
}

// GetDeleteRequests - GET /api/delete-requests
func GetDeleteRequests(c *fiber.Ctx) error {
	status := c.Query("status", "pending")
	rows, err := DB.Query(`
		SELECT id, book_id, judul_snapshot, alasan, requested_by, requested_by_nama,
		       status, reviewed_by, reviewed_at, catatan_review, created_at
		FROM delete_requests WHERE status = $1 ORDER BY created_at DESC
	`, status)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type DeleteReqRow struct {
		ID              int        `json:"id"`
		BookID          int        `json:"book_id"`
		JudulSnapshot   string     `json:"judul_snapshot"`
		Alasan          string     `json:"alasan"`
		RequestedBy     int        `json:"requested_by"`
		RequestedByNama string     `json:"requested_by_nama"`
		Status          string     `json:"status"`
		ReviewedBy      *int       `json:"reviewed_by"`
		ReviewedAt      *time.Time `json:"reviewed_at"`
		CatatanReview   *string    `json:"catatan_review"`
		CreatedAt       time.Time  `json:"created_at"`
	}
	result := []DeleteReqRow{}
	for rows.Next() {
		var r DeleteReqRow
		rows.Scan(&r.ID, &r.BookID, &r.JudulSnapshot, &r.Alasan,
			&r.RequestedBy, &r.RequestedByNama, &r.Status,
			&r.ReviewedBy, &r.ReviewedAt, &r.CatatanReview, &r.CreatedAt)
		result = append(result, r)
	}
	return c.JSON(result)
}

// ApproveDeleteRequest - PUT /api/delete-requests/:id/approve (superadmin)
func ApproveDeleteRequest(c *fiber.Ctx) error {
	id := c.Params("id")
	var input struct {
		AdminID   int    `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	c.BodyParser(&input)

	var bookID int
	var judulSnapshot, status string
	err := DB.QueryRow(`SELECT book_id, judul_snapshot, status FROM delete_requests WHERE id = $1`, id).
		Scan(&bookID, &judulSnapshot, &status)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Pengajuan tidak ditemukan"})
	}
	if status != "pending" {
		return c.Status(409).JSON(fiber.Map{"error": "Pengajuan sudah diproses"})
	}

	tx, err := DB.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal memulai transaksi"})
	}

	_, err = tx.Exec(`UPDATE delete_requests SET status='approved', reviewed_by=$1, reviewed_at=NOW() WHERE id=$2`,
		input.AdminID, id)
	if err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengupdate status pengajuan"})
	}

	_, err = tx.Exec(`DELETE FROM books WHERE id = $1`, bookID)
	if err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": "Gagal menghapus buku"})
	}

	if err = tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	LogActivity(DB, &input.AdminID, input.AdminNama, "DELETE", "book", &bookID, &judulSnapshot, nil)
	return c.JSON(fiber.Map{"message": "Buku berhasil dihapus"})
}

// RejectDeleteRequest - PUT /api/delete-requests/:id/reject (superadmin)
func RejectDeleteRequest(c *fiber.Ctx) error {
	id := c.Params("id")
	var input struct {
		AdminID       int    `json:"admin_id"`
		AdminNama     string `json:"admin_nama"`
		CatatanReview string `json:"catatan_review"`
	}
	c.BodyParser(&input)

	var judulSnapshot, status string
	err := DB.QueryRow("SELECT judul_snapshot, status FROM delete_requests WHERE id = $1", id).
		Scan(&judulSnapshot, &status)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Pengajuan tidak ditemukan"})
	}
	if status != "pending" {
		return c.Status(409).JSON(fiber.Map{"error": "Pengajuan sudah diproses"})
	}

	DB.Exec(`UPDATE delete_requests SET status='rejected', reviewed_by=$1, reviewed_at=NOW(), catatan_review=$2 WHERE id=$3`,
		input.AdminID, nullStr(input.CatatanReview), id)

	LogActivity(DB, &input.AdminID, input.AdminNama, "REJECT_DELETE", "book", nil, &judulSnapshot, nil)
	return c.JSON(fiber.Map{"message": "Pengajuan hapus ditolak"})
}

// ── HELPER ───────────────────────────────────────────────────────────────────
func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
