package handlers

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
)

// GetCategories - GET /api/categories (public)
func GetCategories(c *fiber.Ctx) error {
	rows, err := DB.Query(`
		SELECT c.id, c.nama, COUNT(b.id) AS book_count
		FROM categories c
		LEFT JOIN books b ON b.kategori_id = c.id
		GROUP BY c.id, c.nama
		ORDER BY c.nama ASC
	`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil kategori"})
	}
	defer rows.Close()

	type Category struct {
		ID        int    `json:"id"`
		Nama      string `json:"nama"`
		BookCount int    `json:"book_count"`
	}

	list := []Category{}
	for rows.Next() {
		var cat Category
		if err := rows.Scan(&cat.ID, &cat.Nama, &cat.BookCount); err != nil {
			continue
		}
		list = append(list, cat)
	}
	return c.JSON(list)
}

// CreateCategory - POST /api/categories (superadmin only)
func CreateCategory(c *fiber.Ctx) error {
	var input struct {
		Nama      string `json:"nama"`
		AdminID   int    `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}
	if input.Nama == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Nama kategori wajib diisi"})
	}

	var exists bool
	_ = DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM categories WHERE LOWER(nama) = LOWER($1))`,
		input.Nama).Scan(&exists)
	if exists {
		return c.Status(409).JSON(fiber.Map{"error": "Kategori dengan nama ini sudah ada"})
	}

	var id int
	err := DB.QueryRow(`INSERT INTO categories (nama) VALUES ($1) RETURNING id`, input.Nama).Scan(&id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal membuat kategori"})
	}

	return c.Status(201).JSON(fiber.Map{"message": "Kategori berhasil dibuat", "id": id, "nama": input.Nama})
}

// UpdateCategory - PUT /api/categories/:id
func UpdateCategory(c *fiber.Ctx) error {
	categoryID := c.Params("id")
	var input struct {
		Nama      string `json:"nama"`
		AdminID   int    `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}
	if input.Nama == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Nama kategori wajib diisi"})
	}

	var oldNama string
	err := DB.QueryRow(`SELECT nama FROM categories WHERE id = $1`, categoryID).Scan(&oldNama)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Kategori tidak ditemukan"})
	}

	var exists bool
	_ = DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM categories WHERE LOWER(nama) = LOWER($1) AND id != $2)`,
		input.Nama, categoryID).Scan(&exists)
	if exists {
		return c.Status(409).JSON(fiber.Map{"error": "Nama kategori sudah dipakai"})
	}

	_, err = DB.Exec(`UPDATE categories SET nama = $1 WHERE id = $2`, input.Nama, categoryID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengupdate kategori"})
	}

	return c.JSON(fiber.Map{"message": "Kategori diperbarui", "id": categoryID, "nama": input.Nama, "old_nama": oldNama})
}

// DeleteCategory - DELETE /api/categories/:id (superadmin only)
func DeleteCategory(c *fiber.Ctx) error {
	categoryID := c.Params("id")

	var bookCount int
	_ = DB.QueryRow(`SELECT COUNT(*) FROM books WHERE kategori_id = $1`, categoryID).Scan(&bookCount)
	if bookCount > 0 {
		return c.Status(409).JSON(fiber.Map{
			"error":      "Kategori tidak bisa dihapus karena masih dipakai oleh buku",
			"book_count": bookCount,
		})
	}

	var nama string
	err := DB.QueryRow(`DELETE FROM categories WHERE id = $1 RETURNING nama`, categoryID).Scan(&nama)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Kategori tidak ditemukan"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal menghapus kategori"})
	}

	return c.JSON(fiber.Map{"message": "Kategori berhasil dihapus", "nama": nama})
}

// ─────────────────────────────────────────────
// CATEGORY REQUESTS (Admin → Superadmin approval)
// ─────────────────────────────────────────────

// CreateCategoryRequest - POST /api/category-requests
func CreateCategoryRequest(c *fiber.Ctx) error {
	var input struct {
		NamaRequested   string `json:"nama_requested"`
		Alasan          string `json:"alasan"`
		RequestedBy     int    `json:"requested_by"`
		RequestedByNama string `json:"requested_by_nama"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}
	if input.NamaRequested == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Nama kategori yang diusulkan wajib diisi"})
	}

	var id int
	err := DB.QueryRow(`
		INSERT INTO category_requests (nama_requested, alasan, requested_by, requested_by_nama, status)
		VALUES ($1, $2, NULLIF($3, 0), $4, 'pending') RETURNING id
	`, input.NamaRequested, input.Alasan, input.RequestedBy, input.RequestedByNama).Scan(&id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengajukan kategori"})
	}

	return c.Status(201).JSON(fiber.Map{"message": "Pengajuan kategori berhasil dikirim", "id": id})
}

// GetCategoryRequests - GET /api/category-requests?status=pending
func GetCategoryRequests(c *fiber.Ctx) error {
	status := c.Query("status", "pending")

	rows, err := DB.Query(`
		SELECT id, nama_requested, alasan, requested_by_nama, status,
		       reviewed_by_nama, catatan_review, created_at
		FROM category_requests
		WHERE status = $1
		ORDER BY created_at DESC
	`, status)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil data"})
	}
	defer rows.Close()

	type CatReq struct {
		ID              int     `json:"id"`
		NamaRequested   string  `json:"nama_requested"`
		Alasan          *string `json:"alasan"`
		RequestedByNama string  `json:"requested_by_nama"`
		Status          string  `json:"status"`
		ReviewedByNama  *string `json:"reviewed_by_nama"`
		CatatanReview   *string `json:"catatan_review"`
		CreatedAt       string  `json:"created_at"`
	}

	list := []CatReq{}
	for rows.Next() {
		var r CatReq
		if err := rows.Scan(
			&r.ID, &r.NamaRequested, &r.Alasan, &r.RequestedByNama,
			&r.Status, &r.ReviewedByNama, &r.CatatanReview, &r.CreatedAt,
		); err != nil {
			continue
		}
		list = append(list, r)
	}
	return c.JSON(list)
}

// ApproveCategoryRequest - PUT /api/category-requests/:id/approve
func ApproveCategoryRequest(c *fiber.Ctx) error {
	reqID := c.Params("id")
	var input struct {
		AdminID   int    `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}

	var nama, status string
	err := DB.QueryRow(`SELECT nama_requested, status FROM category_requests WHERE id = $1`, reqID).
		Scan(&nama, &status)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Pengajuan tidak ditemukan"})
	}
	if status != "pending" {
		return c.Status(409).JSON(fiber.Map{"error": "Pengajuan ini sudah diproses"})
	}

	var exists bool
	_ = DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM categories WHERE LOWER(nama) = LOWER($1))`, nama).Scan(&exists)
	if exists {
		return c.Status(409).JSON(fiber.Map{"error": "Kategori dengan nama ini sudah ada"})
	}

	var catID int
	_ = DB.QueryRow(`INSERT INTO categories (nama) VALUES ($1) RETURNING id`, nama).Scan(&catID)

	_, _ = DB.Exec(`
		UPDATE category_requests
		SET status = 'approved', reviewed_by = $1, reviewed_by_nama = $2, reviewed_at = NOW()
		WHERE id = $3
	`, input.AdminID, input.AdminNama, reqID)

	return c.JSON(fiber.Map{
		"message":     "Pengajuan disetujui, kategori baru dibuat",
		"category_id": catID,
		"nama":        nama,
	})
}

// RejectCategoryRequest - PUT /api/category-requests/:id/reject
func RejectCategoryRequest(c *fiber.Ctx) error {
	reqID := c.Params("id")
	var input struct {
		AdminID       int    `json:"admin_id"`
		AdminNama     string `json:"admin_nama"`
		CatatanReview string `json:"catatan_review"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}

	var status string
	err := DB.QueryRow(`SELECT status FROM category_requests WHERE id = $1`, reqID).Scan(&status)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Pengajuan tidak ditemukan"})
	}
	if status != "pending" {
		return c.Status(409).JSON(fiber.Map{"error": "Pengajuan ini sudah diproses"})
	}

	_, _ = DB.Exec(`
		UPDATE category_requests
		SET status = 'rejected', reviewed_by = $1, reviewed_by_nama = $2,
		    reviewed_at = NOW(), catatan_review = $3
		WHERE id = $4
	`, input.AdminID, input.AdminNama, input.CatatanReview, reqID)

	return c.JSON(fiber.Map{"message": "Pengajuan ditolak"})
}
