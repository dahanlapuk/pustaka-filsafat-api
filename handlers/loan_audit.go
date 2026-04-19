package handlers

import (
	"github.com/gofiber/fiber/v2"
)

func GetLoanAuditSummary(c *fiber.Ctx) error {
	var totalLoans int
	var activeLoans int
	var returnedLoans int

	if err := DB.QueryRow(`SELECT COUNT(*) FROM loans`).Scan(&totalLoans); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if err := DB.QueryRow(`SELECT COUNT(*) FROM loans WHERE tanggal_kembali IS NULL`).Scan(&activeLoans); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	returnedLoans = totalLoans - activeLoans

	rows, err := DB.Query(`
		SELECT p.id, p.kode, p.rak, COUNT(*) AS total
		FROM loan_stock_allocations a
		LEFT JOIN posisi p ON p.id = a.posisi_id
		GROUP BY p.id, p.kode, p.rak
		ORDER BY total DESC
	`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	byPosisi := []fiber.Map{}
	for rows.Next() {
		var posisiID *int
		var kode, rak *string
		var total int
		if err := rows.Scan(&posisiID, &kode, &rak, &total); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		byPosisi = append(byPosisi, fiber.Map{
			"posisi_id":   posisiID,
			"posisi_kode": kode,
			"posisi_rak":  rak,
			"total":       total,
		})
	}

	return c.JSON(fiber.Map{
		"total_loans":    totalLoans,
		"active_loans":   activeLoans,
		"returned_loans": returnedLoans,
		"by_posisi":      byPosisi,
	})
}

func GetLoanAuditHistory(c *fiber.Ctx) error {
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")
	posisiID := c.Query("posisi_id")
	bookID := c.Query("book_id")

	query := `
		SELECT
			l.id,
			l.book_id,
			b.judul,
			l.nama_peminjam,
			l.tanggal_pinjam,
			l.tanggal_kembali,
			a.posisi_id,
			p.kode,
			a.qty,
			a.allocated_at,
			a.returned_at
		FROM loans l
		JOIN books b ON b.id = l.book_id
		LEFT JOIN loan_stock_allocations a ON a.loan_id = l.id
		LEFT JOIN posisi p ON p.id = a.posisi_id
		WHERE 1=1
	`
	args := []interface{}{}
	argN := 1
	if dateFrom != "" {
		query += " AND l.tanggal_pinjam >= $" + itoa(argN)
		args = append(args, dateFrom)
		argN++
	}
	if dateTo != "" {
		query += " AND l.tanggal_pinjam <= $" + itoa(argN)
		args = append(args, dateTo)
		argN++
	}
	if posisiID != "" {
		query += " AND a.posisi_id::text = $" + itoa(argN)
		args = append(args, posisiID)
		argN++
	}
	if bookID != "" {
		query += " AND l.book_id::text = $" + itoa(argN)
		args = append(args, bookID)
		argN++
	}
	query += " ORDER BY l.tanggal_pinjam DESC LIMIT 500"

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	history := []fiber.Map{}
	for rows.Next() {
		var loanID, bID int
		var title, borrower string
		var pinjam string
		var kembali *string
		var allocPosisiID *int
		var posisiKode *string
		var qty *int
		var allocatedAt *string
		var returnedAt *string
		if err := rows.Scan(&loanID, &bID, &title, &borrower, &pinjam, &kembali, &allocPosisiID, &posisiKode, &qty, &allocatedAt, &returnedAt); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		history = append(history, fiber.Map{
			"loan_id":         loanID,
			"book_id":         bID,
			"book_title":      title,
			"borrower":        borrower,
			"tanggal_pinjam":  pinjam,
			"tanggal_kembali": kembali,
			"posisi_id":       allocPosisiID,
			"posisi_kode":     posisiKode,
			"qty":             qty,
			"allocated_at":    allocatedAt,
			"returned_at":     returnedAt,
		})
	}

	return c.JSON(fiber.Map{"history": history})
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	return string(buf[i:])
}
