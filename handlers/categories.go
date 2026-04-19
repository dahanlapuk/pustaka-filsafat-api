package handlers

import (
	"database/sql"
	"pustaka-filsafat/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
)

// GetCategories - GET /api/categories (public)
func GetCategories(c *fiber.Ctx) error {
	rows, err := DB.Query(`
		SELECT c.id, c.nama, c.grouping, COUNT(b.id) AS book_count
		FROM categories c
		LEFT JOIN books b ON b.kategori_id = c.id
		GROUP BY c.id, c.nama, c.grouping
		ORDER BY c.nama ASC
	`)
	useGrouping := true
	if err != nil && isUndefinedColumnCategoryError(err) {
		useGrouping = false
		rows, err = DB.Query(`
			SELECT c.id, c.nama, COUNT(b.id) AS book_count
			FROM categories c
			LEFT JOIN books b ON b.kategori_id = c.id
			GROUP BY c.id, c.nama
			ORDER BY c.nama ASC
		`)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil kategori"})
	}
	defer rows.Close()

	type Category struct {
		ID        int     `json:"id"`
		Nama      string  `json:"nama"`
		Grouping  *string `json:"grouping,omitempty"`
		BookCount int     `json:"book_count"`
	}

	list := []Category{}
	for rows.Next() {
		var cat Category
		if useGrouping {
			if err := rows.Scan(&cat.ID, &cat.Nama, &cat.Grouping, &cat.BookCount); err != nil {
				continue
			}
		} else {
			if err := rows.Scan(&cat.ID, &cat.Nama, &cat.BookCount); err != nil {
				continue
			}
		}
		if !useGrouping {
			cat.Grouping = nil
		}
		list = append(list, cat)
	}
	if err := rows.Err(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengambil kategori"})
	}
	return c.JSON(list)
}

// CreateCategory - POST /api/categories (superadmin only)
func CreateCategory(c *fiber.Ctx) error {
	var input struct {
		Nama      string `json:"nama"`
		Grouping  string `json:"grouping"`
		AdminID   int    `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}
	if input.Nama == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Nama kategori wajib diisi"})
	}

	normalizedGrouping := normalizeCategoryGrouping(input.Grouping)

	var exists bool
	_ = DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM categories WHERE LOWER(nama) = LOWER($1))`,
		input.Nama).Scan(&exists)
	if exists {
		return c.Status(409).JSON(fiber.Map{"error": "Kategori dengan nama ini sudah ada"})
	}

	var id int
	err := DB.QueryRow(`INSERT INTO categories (nama, grouping) VALUES ($1, NULLIF($2, '')) RETURNING id`, input.Nama, normalizedGrouping).Scan(&id)
	if err != nil && isUndefinedColumnCategoryError(err) {
		err = DB.QueryRow(`INSERT INTO categories (nama) VALUES ($1) RETURNING id`, input.Nama).Scan(&id)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal membuat kategori"})
	}

	entityName := input.Nama
	_ = LogActivity(DB, &input.AdminID, input.AdminNama, models.ActionCreate, models.EntityCategory, &id, &entityName, map[string]interface{}{
		"nama":     input.Nama,
		"grouping": normalizedGrouping,
	})

	return c.Status(201).JSON(fiber.Map{"message": "Kategori berhasil dibuat", "id": id, "nama": input.Nama, "grouping": normalizedGrouping})
}

// UpdateCategory - PUT /api/categories/:id
func UpdateCategory(c *fiber.Ctx) error {
	categoryID := c.Params("id")
	var input struct {
		Nama      string `json:"nama"`
		Grouping  string `json:"grouping"`
		AdminID   int    `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}
	if input.Nama == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Nama kategori wajib diisi"})
	}

	normalizedGrouping := normalizeCategoryGrouping(input.Grouping)

	var oldNama string
	var oldGrouping sql.NullString
	err := DB.QueryRow(`SELECT nama, grouping FROM categories WHERE id = $1`, categoryID).Scan(&oldNama, &oldGrouping)
	useGrouping := true
	if err != nil && isUndefinedColumnCategoryError(err) {
		useGrouping = false
		err = DB.QueryRow(`SELECT nama FROM categories WHERE id = $1`, categoryID).Scan(&oldNama)
		oldGrouping = sql.NullString{}
	}
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Kategori tidak ditemukan"})
	}

	var exists bool
	_ = DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM categories WHERE LOWER(nama) = LOWER($1) AND id != $2)`,
		input.Nama, categoryID).Scan(&exists)
	if exists {
		return c.Status(409).JSON(fiber.Map{"error": "Nama kategori sudah dipakai"})
	}

	if useGrouping {
		_, err = DB.Exec(`UPDATE categories SET nama = $1, grouping = NULLIF($2, '') WHERE id = $3`, input.Nama, normalizedGrouping, categoryID)
		if err != nil && isUndefinedColumnCategoryError(err) {
			useGrouping = false
			_, err = DB.Exec(`UPDATE categories SET nama = $1 WHERE id = $2`, input.Nama, categoryID)
		}
	} else {
		_, err = DB.Exec(`UPDATE categories SET nama = $1 WHERE id = $2`, input.Nama, categoryID)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mengupdate kategori"})
	}

	entityName := input.Nama
	_ = LogActivity(DB, &input.AdminID, input.AdminNama, "CATEGORY_UPDATE_REQUEST", models.EntityCategory, intPtr(categoryID), &entityName, map[string]interface{}{
		"old_nama":     oldNama,
		"new_nama":     input.Nama,
		"old_grouping": oldGrouping.String,
		"new_grouping": normalizedGrouping,
		"category_id":  categoryID,
	})

	return c.JSON(fiber.Map{"message": "Kategori diperbarui", "id": categoryID, "nama": input.Nama, "grouping": normalizedGrouping, "old_nama": oldNama})
}

func isUndefinedColumnCategoryError(err error) bool {
	pqErr, ok := err.(*pq.Error)
	return ok && pqErr.Code == "42703"
}

func normalizeCategoryGrouping(grouping string) string {
	switch grouping {
	case "bentuk", "konten", "lain":
		return grouping
	default:
		return ""
	}
}

// DeleteCategory - DELETE /api/categories/:id (superadmin only)
func DeleteCategory(c *fiber.Ctx) error {
	categoryID := c.Params("id")
	var input struct {
		AdminID   int    `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	_ = c.BodyParser(&input)

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

	_ = LogActivity(DB, &input.AdminID, input.AdminNama, "CATEGORY_DELETE_APPROVE", models.EntityCategory, intPtr(categoryID), &nama, map[string]interface{}{
		"category_id": categoryID,
		"nama":        nama,
	})

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

	entityName := input.NamaRequested
	_ = LogActivity(DB, &input.RequestedBy, input.RequestedByNama, "CATEGORY_REQUEST_CREATE", "CATEGORY_REQUEST", &id, &entityName, map[string]interface{}{
		"nama_requested": input.NamaRequested,
		"alasan":         input.Alasan,
	})

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

	entityName := nama
	_ = LogActivity(DB, &input.AdminID, input.AdminNama, "CATEGORY_REQUEST_APPROVE", "CATEGORY_REQUEST", nil, &entityName, map[string]interface{}{
		"request_id":  reqID,
		"category_id": catID,
		"nama":        nama,
	})
	_ = LogActivity(DB, &input.AdminID, input.AdminNama, "CATEGORY_CREATE_APPROVE", models.EntityCategory, &catID, &entityName, map[string]interface{}{
		"request_id": reqID,
	})

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

	_ = LogActivity(DB, &input.AdminID, input.AdminNama, "CATEGORY_REQUEST_REJECT", "CATEGORY_REQUEST", nil, nil, map[string]interface{}{
		"request_id":     reqID,
		"catatan_review": input.CatatanReview,
	})

	return c.JSON(fiber.Map{"message": "Pengajuan ditolak"})
}

func intPtr(s string) *int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}
