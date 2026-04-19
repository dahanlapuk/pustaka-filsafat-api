package handlers

import (
	"database/sql"
	"log"
	"time"

	"pustaka-filsafat/models"

	"github.com/gofiber/fiber/v2"
)

// TransferInventoryRequest adalah request untuk memindahkan stok antar posisi
type TransferInventoryRequest struct {
	BookID     int    `json:"book_id" validate:"required"`
	FromPosisi int    `json:"from_posisi" validate:"required"`
	ToPosisi   int    `json:"to_posisi" validate:"required"`
	Quantity   int    `json:"quantity" validate:"required,gt=0"`
	Notes      string `json:"notes"`
}

// TransferInventoryResponse adalah response untuk transfer inventory
type TransferInventoryResponse struct {
	Success    bool            `json:"success"`
	Message    string          `json:"message"`
	Allocation []StockLocation `json:"allocation,omitempty"`
	TransferID string          `json:"transfer_id,omitempty"`
}

// TransferInventory memindahkan stok dari satu posisi ke posisi lain
// POST /inventory/transfer
func TransferInventory(c *fiber.Ctx) error {
	var adminIDPtr *int
	adminNama := "System"
	if raw := c.Locals("adminID"); raw != nil {
		if id, ok := raw.(int); ok {
			adminIDPtr = &id
			if err := DB.QueryRow("SELECT nama FROM admins WHERE id = $1", id).Scan(&adminNama); err != nil {
				adminNama = "Admin"
			}
		}
	}

	var req TransferInventoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	// Validasi: from != to
	if req.FromPosisi == req.ToPosisi {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Posisi asal dan tujuan tidak boleh sama"})
	}

	// Validasi: qty > 0
	if req.Quantity <= 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Jumlah harus lebih dari 0"})
	}

	// Cek ketersediaan stok di posisi sumber
	var currentQty int
	err := DB.QueryRow(`
		SELECT COALESCE(quantity, 0) FROM book_stock_locations 
		WHERE book_id = $1 AND posisi_id = $2
	`, req.BookID, req.FromPosisi).Scan(&currentQty)
	if err == sql.ErrNoRows {
		currentQty = 0
		err = nil
	}
	if err != nil {
		log.Printf("[TransferInventory] Error cek stok: %v", err)
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Gagal cek stok"})
	}

	if currentQty < req.Quantity {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Stok tidak cukup di posisi sumber",
		})
	}

	// Mulai transaction
	tx, err := DB.Begin()
	if err != nil {
		log.Printf("[TransferInventory] Error begin tx: %v", err)
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Transaction error"})
	}
	defer tx.Rollback()

	// Update stok di posisi sumber (kurangi)
	_, err = tx.Exec(`
		UPDATE book_stock_locations 
		SET quantity = quantity - $1, updated_at = NOW()
		WHERE book_id = $2 AND posisi_id = $3
	`, req.Quantity, req.BookID, req.FromPosisi)
	if err != nil {
		log.Printf("[TransferInventory] Error update source: %v", err)
		tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Update source failed"})
	}

	// Upsert stok di posisi tujuan (tambah)
	_, err = tx.Exec(`
		INSERT INTO book_stock_locations (book_id, posisi_id, quantity, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (book_id, posisi_id) 
		DO UPDATE SET quantity = quantity + $3, updated_at = NOW()
	`, req.BookID, req.ToPosisi, req.Quantity)
	if err != nil {
		log.Printf("[TransferInventory] Error update dest: %v", err)
		tx.Rollback()
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Update dest failed"})
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Printf("[TransferInventory] Error commit: %v", err)
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Commit failed"})
	}

	// Log activity setelah commit agar tidak menggagalkan transfer inti
	if err := LogActivity(DB, adminIDPtr, adminNama, "INVENTORY_TRANSFER", models.EntityBook, &req.BookID, nil, map[string]interface{}{
		"from_posisi": req.FromPosisi,
		"to_posisi":   req.ToPosisi,
		"qty":         req.Quantity,
		"notes":       req.Notes,
	}); err != nil {
		log.Printf("[TransferInventory] Error log: %v", err)
	}

	// Fetch updated allocations
	var allocations []StockLocation
	rows, err := DB.Query(`
		SELECT posisi_id, quantity FROM book_stock_locations 
		WHERE book_id = $1 AND quantity > 0
		ORDER BY posisi_id
	`, req.BookID)
	if err != nil {
		log.Printf("[TransferInventory] Error fetch allocations: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var loc StockLocation
			rows.Scan(&loc.PosisiID, &loc.Quantity)
			allocations = append(allocations, loc)
		}
	}

	return c.Status(200).JSON(TransferInventoryResponse{
		Success:    true,
		Message:    "Transfer berhasil",
		TransferID: time.Now().Format("20060102150405"),
		Allocation: allocations,
	})
}

// StockLocation adalah struktur untuk lokasi stok buku
type StockLocation struct {
	PosisiID int `json:"posisi_id"`
	Quantity int `json:"quantity"`
}
