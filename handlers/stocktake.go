package handlers

import (
	"database/sql"
	"fmt"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v2"
)

var ensureStocktakeSchemaOnce sync.Once
var ensureStocktakeSchemaErr error

func EnsureStocktakeSchema(db *sql.DB) error {
	ensureStocktakeSchemaOnce.Do(func() {
		stmts := []string{
			`CREATE TABLE IF NOT EXISTS stocktake_sessions (
				id BIGSERIAL PRIMARY KEY,
				session_code TEXT UNIQUE NOT NULL,
				status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
				created_by INT REFERENCES admins(id) ON DELETE SET NULL,
				created_at TIMESTAMP NOT NULL DEFAULT NOW(),
				closed_at TIMESTAMP NULL
			)`,
			`CREATE TABLE IF NOT EXISTS stocktake_entries (
				id BIGSERIAL PRIMARY KEY,
				session_id BIGINT NOT NULL REFERENCES stocktake_sessions(id) ON DELETE CASCADE,
				book_id INT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
				posisi_id INT REFERENCES posisi(id) ON DELETE SET NULL,
				system_qty INT NOT NULL DEFAULT 0,
				physical_qty INT NOT NULL DEFAULT 0,
				discrepancy INT NOT NULL DEFAULT 0,
				notes TEXT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_stocktake_entries_session ON stocktake_entries(session_id)`,
			`CREATE INDEX IF NOT EXISTS idx_stocktake_entries_book_posisi ON stocktake_entries(book_id, posisi_id)`,
		}
		for _, stmt := range stmts {
			if _, err := db.Exec(stmt); err != nil {
				ensureStocktakeSchemaErr = fmt.Errorf("gagal memastikan skema stocktake: %w", err)
				return
			}
		}
	})
	return ensureStocktakeSchemaErr
}

type stocktakeSessionCreateInput struct {
	SessionCode string `json:"session_code"`
}

type stocktakeEntryInput struct {
	BookID      int    `json:"book_id"`
	PosisiID    *int   `json:"posisi_id"`
	PhysicalQty int    `json:"physical_qty"`
	Notes       string `json:"notes"`
}

func StartStocktakeSession(c *fiber.Ctx) error {
	if err := EnsureStocktakeSchema(DB); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	var input stocktakeSessionCreateInput
	_ = c.BodyParser(&input)
	if input.SessionCode == "" {
		input.SessionCode = "STOCKTAKE-" + c.Get("X-Request-Id")
		if input.SessionCode == "STOCKTAKE-" {
			input.SessionCode = "STOCKTAKE-AUTO"
		}
	}

	var createdBy *int
	if raw := c.Locals("adminID"); raw != nil {
		if id, ok := raw.(int); ok {
			createdBy = &id
		}
	}

	var id int64
	err := DB.QueryRow(`
		INSERT INTO stocktake_sessions (session_code, created_by)
		VALUES ($1, $2)
		RETURNING id
	`, input.SessionCode, createdBy).Scan(&id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal membuat sesi stocktake"})
	}

	return c.Status(201).JSON(fiber.Map{
		"id":           id,
		"session_code": input.SessionCode,
		"status":       "open",
	})
}

func AddStocktakeEntry(c *fiber.Ctx) error {
	if err := EnsureStocktakeSchema(DB); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	sessionID, err := strconv.Atoi(c.Params("id"))
	if err != nil || sessionID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Session ID tidak valid"})
	}

	var input stocktakeEntryInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}
	if input.BookID <= 0 || input.PhysicalQty < 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Book ID / qty fisik tidak valid"})
	}

	var status string
	if err := DB.QueryRow(`SELECT status FROM stocktake_sessions WHERE id = $1`, sessionID).Scan(&status); err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Session tidak ditemukan"})
	} else if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if status != "open" {
		return c.Status(400).JSON(fiber.Map{"error": "Session sudah ditutup"})
	}

	var systemQty int
	err = DB.QueryRow(`
		SELECT COALESCE(SUM(qty), 0)
		FROM book_stock_locations
		WHERE book_id = $1 AND posisi_id IS NOT DISTINCT FROM $2
	`, input.BookID, input.PosisiID).Scan(&systemQty)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal menghitung stok sistem"})
	}
	discrepancy := input.PhysicalQty - systemQty

	var entryID int64
	err = DB.QueryRow(`
		INSERT INTO stocktake_entries (session_id, book_id, posisi_id, system_qty, physical_qty, discrepancy, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, sessionID, input.BookID, input.PosisiID, systemQty, input.PhysicalQty, discrepancy, input.Notes).Scan(&entryID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal menyimpan entry stocktake"})
	}

	return c.Status(201).JSON(fiber.Map{
		"id":           entryID,
		"session_id":   sessionID,
		"system_qty":   systemQty,
		"physical_qty": input.PhysicalQty,
		"discrepancy":  discrepancy,
	})
}

func GetStocktakeSession(c *fiber.Ctx) error {
	sessionID, err := strconv.Atoi(c.Params("id"))
	if err != nil || sessionID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Session ID tidak valid"})
	}

	var session struct {
		ID          int64   `json:"id"`
		SessionCode string  `json:"session_code"`
		Status      string  `json:"status"`
		CreatedAt   string  `json:"created_at"`
		ClosedAt    *string `json:"closed_at,omitempty"`
	}

	err = DB.QueryRow(`
		SELECT id, session_code, status, created_at::text, closed_at::text
		FROM stocktake_sessions
		WHERE id = $1
	`, sessionID).Scan(&session.ID, &session.SessionCode, &session.Status, &session.CreatedAt, &session.ClosedAt)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Session tidak ditemukan"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	rows, err := DB.Query(`
		SELECT e.id, e.book_id, b.judul, e.posisi_id, p.kode, e.system_qty, e.physical_qty, e.discrepancy, e.notes, e.created_at::text
		FROM stocktake_entries e
		JOIN books b ON b.id = e.book_id
		LEFT JOIN posisi p ON p.id = e.posisi_id
		WHERE e.session_id = $1
		ORDER BY e.created_at DESC
	`, sessionID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	entries := []fiber.Map{}
	for rows.Next() {
		var id, bookID, systemQty, physicalQty, discrepancy int
		var title, createdAt string
		var posisiID *int
		var posisiKode *string
		var notes *string
		if err := rows.Scan(&id, &bookID, &title, &posisiID, &posisiKode, &systemQty, &physicalQty, &discrepancy, &notes, &createdAt); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		entries = append(entries, fiber.Map{
			"id":           id,
			"book_id":      bookID,
			"book_title":   title,
			"posisi_id":    posisiID,
			"posisi_kode":  posisiKode,
			"system_qty":   systemQty,
			"physical_qty": physicalQty,
			"discrepancy":  discrepancy,
			"notes":        notes,
			"created_at":   createdAt,
		})
	}

	return c.JSON(fiber.Map{
		"session": session,
		"entries": entries,
	})
}

func CloseStocktakeSession(c *fiber.Ctx) error {
	sessionID, err := strconv.Atoi(c.Params("id"))
	if err != nil || sessionID <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Session ID tidak valid"})
	}

	result, err := DB.Exec(`
		UPDATE stocktake_sessions
		SET status = 'closed', closed_at = NOW()
		WHERE id = $1 AND status = 'open'
	`, sessionID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Session tidak ditemukan atau sudah ditutup"})
	}

	var summary struct {
		TotalEntries     int `json:"total_entries"`
		TotalDiscrepancy int `json:"total_discrepancy"`
	}
	_ = DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(discrepancy),0)
		FROM stocktake_entries
		WHERE session_id = $1
	`, sessionID).Scan(&summary.TotalEntries, &summary.TotalDiscrepancy)

	return c.JSON(fiber.Map{
		"message": "Session stocktake ditutup",
		"summary": summary,
	})
}
