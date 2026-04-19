package handlers

import (
	"database/sql"
	"fmt"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v2"
)

var ensureBookStockSchemaOnce sync.Once
var ensureBookStockSchemaErr error

func EnsureBookStockSchema(db *sql.DB) error {
	ensureBookStockSchemaOnce.Do(func() {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS book_stock_locations (
				id BIGSERIAL PRIMARY KEY,
				book_id INT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
				posisi_id INT REFERENCES posisi(id) ON DELETE SET NULL,
				qty INT NOT NULL DEFAULT 0 CHECK (qty >= 0),
				created_at TIMESTAMP NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
				UNIQUE(book_id, posisi_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_book_stock_locations_book_id ON book_stock_locations(book_id)`,
			`CREATE INDEX IF NOT EXISTS idx_book_stock_locations_posisi_id ON book_stock_locations(posisi_id)`,
			`CREATE TABLE IF NOT EXISTS loan_stock_allocations (
				id BIGSERIAL PRIMARY KEY,
				loan_id INT NOT NULL REFERENCES loans(id) ON DELETE CASCADE,
				book_id INT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
				posisi_id INT REFERENCES posisi(id) ON DELETE SET NULL,
				qty INT NOT NULL DEFAULT 1 CHECK (qty > 0),
				allocated_at TIMESTAMP NOT NULL DEFAULT NOW(),
				returned_at TIMESTAMP NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_loan_stock_allocations_loan_id ON loan_stock_allocations(loan_id)`,
			`CREATE INDEX IF NOT EXISTS idx_loan_stock_allocations_book_posisi_active ON loan_stock_allocations(book_id, posisi_id) WHERE returned_at IS NULL`,
			`INSERT INTO book_stock_locations (book_id, posisi_id, qty)
				SELECT b.id, b.posisi_id, GREATEST(COALESCE(b.qty, 1), 1)
				FROM books b
				WHERE b.posisi_id IS NOT NULL
				ON CONFLICT (book_id, posisi_id) DO UPDATE
				SET qty = EXCLUDED.qty, updated_at = NOW()`,
			`INSERT INTO book_stock_locations (book_id, posisi_id, qty)
				SELECT b.id, NULL, GREATEST(COALESCE(b.qty, 1), 1)
				FROM books b
				WHERE b.posisi_id IS NULL
				  AND NOT EXISTS (
					SELECT 1 FROM book_stock_locations s WHERE s.book_id = b.id
				  )`,
		}

		for _, stmt := range statements {
			if _, err := db.Exec(stmt); err != nil {
				ensureBookStockSchemaErr = fmt.Errorf("gagal memastikan skema split stok: %w", err)
				return
			}
		}
	})

	return ensureBookStockSchemaErr
}

type stockAllocationInput struct {
	PosisiID *int `json:"posisi_id"`
	Qty      int  `json:"qty"`
}

type stockBreakdownInput struct {
	Allocations []stockAllocationInput `json:"allocations"`
	AdminID     *int                   `json:"admin_id"`
	AdminNama   string                 `json:"admin_nama"`
}

func GetBookStockBreakdown(c *fiber.Ctx) error {
	bookID, err := strconv.Atoi(c.Params("id"))
	if err != nil || bookID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Book ID tidak valid"})
	}

	if err := EnsureBookStockSchema(DB); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	var totalQty int
	var posisiID *int
	err = DB.QueryRow(`SELECT qty, posisi_id FROM books WHERE id = $1`, bookID).Scan(&totalQty, &posisiID)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Book not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	rows, err := DB.Query(`
		SELECT s.posisi_id, p.kode, p.rak, s.qty
		FROM book_stock_locations s
		LEFT JOIN posisi p ON p.id = s.posisi_id
		WHERE s.book_id = $1 AND s.qty > 0
		ORDER BY s.qty DESC, p.kode ASC NULLS LAST
	`, bookID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type item struct {
		PosisiID   *int    `json:"posisi_id"`
		PosisiKode *string `json:"posisi_kode,omitempty"`
		PosisiRak  *string `json:"posisi_rak,omitempty"`
		Qty        int     `json:"qty"`
	}

	items := []item{}
	allocated := 0
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.PosisiID, &it.PosisiKode, &it.PosisiRak, &it.Qty); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		allocated += it.Qty
		items = append(items, it)
	}

	return c.JSON(fiber.Map{
		"book_id":        bookID,
		"book_qty":       totalQty,
		"allocated_qty":  allocated,
		"is_consistent":  allocated == totalQty,
		"default_posisi": posisiID,
		"allocations":    items,
	})
}

func UpdateBookStockBreakdown(c *fiber.Ctx) error {
	bookID, err := strconv.Atoi(c.Params("id"))
	if err != nil || bookID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Book ID tidak valid"})
	}

	if err := EnsureBookStockSchema(DB); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	var input stockBreakdownInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}

	tx, err := DB.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal memulai transaksi"})
	}

	var bookQty int
	var bookPosisiID *int
	err = tx.QueryRow(`SELECT qty, posisi_id FROM books WHERE id = $1`, bookID).Scan(&bookQty, &bookPosisiID)
	if err == sql.ErrNoRows {
		_ = tx.Rollback()
		return c.Status(404).JSON(fiber.Map{"error": "Book not found"})
	}
	if err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if bookQty <= 0 {
		bookQty = 1
	}

	total := 0
	for _, a := range input.Allocations {
		if a.Qty < 0 {
			_ = tx.Rollback()
			return c.Status(400).JSON(fiber.Map{"error": "Qty alokasi tidak boleh negatif"})
		}
		total += a.Qty
	}

	if len(input.Allocations) == 0 && bookQty == 1 {
		input.Allocations = []stockAllocationInput{{PosisiID: bookPosisiID, Qty: 1}}
		total = 1
	}

	if total != bookQty {
		_ = tx.Rollback()
		return c.Status(400).JSON(fiber.Map{"error": "Total alokasi harus sama dengan qty buku"})
	}

	if _, err := tx.Exec(`DELETE FROM book_stock_locations WHERE book_id = $1`, bookID); err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	for _, a := range input.Allocations {
		if a.Qty == 0 {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO book_stock_locations (book_id, posisi_id, qty)
			VALUES ($1, $2, $3)
		`, bookID, a.PosisiID, a.Qty); err != nil {
			_ = tx.Rollback()
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	}

	var dominantPosisiID *int
	_ = tx.QueryRow(`
		SELECT posisi_id
		FROM book_stock_locations
		WHERE book_id = $1 AND qty > 0
		ORDER BY qty DESC
		LIMIT 1
	`, bookID).Scan(&dominantPosisiID)

	if _, err := tx.Exec(`UPDATE books SET posisi_id = $1, updated_at = NOW() WHERE id = $2`, dominantPosisiID, bookID); err != nil {
		_ = tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	adminNama := input.AdminNama
	if adminNama == "" {
		adminNama = "System"
	}
	entity := fmt.Sprintf("Book #%d stock breakdown", bookID)
	_ = LogActivity(DB, input.AdminID, adminNama, "BOOK_STOCK_SPLIT_UPDATE", "BOOK_STOCK", &bookID, &entity, map[string]interface{}{
		"book_id":            bookID,
		"book_qty":           bookQty,
		"allocations":        input.Allocations,
		"dominant_posisi_id": dominantPosisiID,
	})

	return c.JSON(fiber.Map{"message": "Distribusi stok berhasil diperbarui", "book_id": bookID})
}

func syncBookStockWithTotalTx(tx *sql.Tx, bookID int, posisiID *int, totalQty int) error {
	if totalQty <= 0 {
		totalQty = 1
	}

	type row struct {
		PosisiID *int
		Qty      int
	}
	rows := []row{}

	qRows, err := tx.Query(`
		SELECT posisi_id, qty
		FROM book_stock_locations
		WHERE book_id = $1
		ORDER BY qty DESC
	`, bookID)
	if err != nil {
		return err
	}
	defer qRows.Close()

	currentTotal := 0
	for qRows.Next() {
		var r row
		if err := qRows.Scan(&r.PosisiID, &r.Qty); err != nil {
			return err
		}
		currentTotal += r.Qty
		rows = append(rows, r)
	}

	if len(rows) == 0 {
		_, err := tx.Exec(`
			INSERT INTO book_stock_locations (book_id, posisi_id, qty)
			VALUES ($1, $2, $3)
		`, bookID, posisiID, totalQty)
		return err
	}

	if len(rows) == 1 && posisiID != nil {
		if _, err := tx.Exec(`
			UPDATE book_stock_locations
			SET posisi_id = $1, qty = $2, updated_at = NOW()
			WHERE book_id = $3
		`, posisiID, totalQty, bookID); err == nil {
			return nil
		}
	}

	delta := totalQty - currentTotal
	if delta > 0 {
		targetPosisi := rows[0].PosisiID
		if posisiID != nil {
			targetPosisi = posisiID
		}
		_, err := tx.Exec(`
			INSERT INTO book_stock_locations (book_id, posisi_id, qty)
			VALUES ($1, $2, $3)
			ON CONFLICT (book_id, posisi_id)
			DO UPDATE SET qty = book_stock_locations.qty + EXCLUDED.qty, updated_at = NOW()
		`, bookID, targetPosisi, delta)
		return err
	}

	if delta < 0 {
		needReduce := -delta
		for _, r := range rows {
			if needReduce == 0 {
				break
			}
			if r.Qty == 0 {
				continue
			}
			reduce := r.Qty
			if reduce > needReduce {
				reduce = needReduce
			}
			newQty := r.Qty - reduce
			if _, err := tx.Exec(`
				UPDATE book_stock_locations
				SET qty = $1, updated_at = NOW()
				WHERE book_id = $2 AND posisi_id IS NOT DISTINCT FROM $3
			`, newQty, bookID, r.PosisiID); err != nil {
				return err
			}
			needReduce -= reduce
		}

		if needReduce > 0 {
			return fmt.Errorf("stok lokasi tidak cukup untuk menyesuaikan qty total")
		}

		if _, err := tx.Exec(`DELETE FROM book_stock_locations WHERE book_id = $1 AND qty <= 0`, bookID); err != nil {
			return err
		}
	}

	return nil
}

func allocateLoanFromLargestStockTx(tx *sql.Tx, bookID int) (*int, error) {
	query := `
		SELECT s.posisi_id
		FROM book_stock_locations s
		LEFT JOIN (
			SELECT a.posisi_id, SUM(a.qty) AS allocated
			FROM loan_stock_allocations a
			WHERE a.book_id = $1 AND a.returned_at IS NULL
			GROUP BY a.posisi_id
		) active ON active.posisi_id IS NOT DISTINCT FROM s.posisi_id
		WHERE s.book_id = $1
		  AND (s.qty - COALESCE(active.allocated, 0)) > 0
		ORDER BY (s.qty - COALESCE(active.allocated, 0)) DESC, s.qty DESC
		LIMIT 1
	`

	var posisiID *int
	err := tx.QueryRow(query, bookID).Scan(&posisiID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return posisiID, nil
}
