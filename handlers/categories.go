package handlers

import (
	"pustaka-filsafat/models"

	"github.com/gofiber/fiber/v2"
)

// GetCategories - GET /api/categories
func GetCategories(c *fiber.Ctx) error {
	rows, err := DB.Query("SELECT id, nama FROM categories ORDER BY nama ASC")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var cat models.Category
		if err := rows.Scan(&cat.ID, &cat.Nama); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		categories = append(categories, cat)
	}

	return c.JSON(categories)
}
