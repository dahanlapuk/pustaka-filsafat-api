package handlers

import (
	"pustaka-filsafat/models"

	"github.com/gofiber/fiber/v2"
)

// GetPosisi - GET /api/posisi
func GetPosisi(c *fiber.Ctx) error {
	rows, err := DB.Query("SELECT id, kode, rak, deskripsi FROM posisi ORDER BY rak, kode ASC")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	posisiList := []models.Posisi{}
	for rows.Next() {
		var p models.Posisi
		if err := rows.Scan(&p.ID, &p.Kode, &p.Rak, &p.Deskripsi); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		posisiList = append(posisiList, p)
	}

	return c.JSON(posisiList)
}
