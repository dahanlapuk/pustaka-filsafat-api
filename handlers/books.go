package handlers

import (
	"database/sql"
	"strconv"

	"pustaka-filsafat/models"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
)

var DB *sql.DB

func SetDB(db *sql.DB) {
	DB = db
}

// GetBooks - GET /api/books?page=1&limit=20
func GetBooks(c *fiber.Ctx) error {
	kategoriID := c.Query("kategori_id")
	posisiID   := c.Query("posisi_id")
	status     := c.Query("status")

	// Pagination
	page  := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	if page  < 1 { page  = 1 }
	if limit < 1 { limit = 10 }
	if limit > 200 { limit = 200 }
	offset := (page - 1) * limit

	baseWhere := " WHERE 1=1"
	args      := []interface{}{}
	argIdx    := 1

	if kategoriID != "" {
		baseWhere += " AND b.kategori_id = $" + strconv.Itoa(argIdx)
		args = append(args, kategoriID)
		argIdx++
	}
	if posisiID != "" {
		baseWhere += " AND b.posisi_id = $" + strconv.Itoa(argIdx)
		args = append(args, posisiID)
		argIdx++
	}
	if status == "dipinjam" {
		baseWhere += " AND l.id IS NOT NULL"
	} else if status == "tersedia" {
		baseWhere += " AND l.id IS NULL"
	}

	countQuery := `
		SELECT COUNT(*)
		FROM books b
		LEFT JOIN categories c ON b.kategori_id = c.id
		LEFT JOIN posisi p     ON b.posisi_id    = p.id
		LEFT JOIN loans l      ON b.id = l.book_id AND l.tanggal_kembali IS NULL
	` + baseWhere

	var total int
	if err := DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	totalPages := (total + limit - 1) / limit

	dataQuery := `
		SELECT
			b.id, b.kode, b.judul, b.kategori_id, b.posisi_id, b.qty, b.keterangan, b.created_at, b.updated_at,
			c.nama as kategori_nama,
			p.kode as posisi_kode, p.rak as posisi_rak,
			CASE WHEN l.id IS NOT NULL THEN true ELSE false END as is_dipinjam,
			l.nama_peminjam as peminjam
		FROM books b
		LEFT JOIN categories c ON b.kategori_id = c.id
		LEFT JOIN posisi p     ON b.posisi_id    = p.id
		LEFT JOIN loans l      ON b.id = l.book_id AND l.tanggal_kembali IS NULL
	` + baseWhere + " ORDER BY b.judul ASC"
	dataQuery += " LIMIT $" + strconv.Itoa(argIdx) + " OFFSET $" + strconv.Itoa(argIdx+1)
	args = append(args, limit, offset)

	rows, err := DB.Query(dataQuery, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	books := []models.Book{}
	for rows.Next() {
		var b models.Book
		if err := rows.Scan(
			&b.ID, &b.Kode, &b.Judul, &b.KategoriID, &b.PosisiID, &b.Qty, &b.Keterangan, &b.CreatedAt, &b.UpdatedAt,
			&b.KategoriNama, &b.PosisiKode, &b.PosisiRak, &b.IsDipinjam, &b.Peminjam,
		); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		books = append(books, b)
	}

	return c.JSON(fiber.Map{
		"data":        books,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

// SearchBooks - GET /api/books/search?q=keyword&page=1&limit=20
func SearchBooks(c *fiber.Ctx) error {
	q := c.Query("q")
	if q == "" {
		return c.JSON(fiber.Map{"data": []models.Book{}, "total": 0, "page": 1, "limit": 20, "total_pages": 0})
	}

	page  := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	if page  < 1 { page  = 1 }
	if limit < 1 { limit = 10 }
	if limit > 200 { limit = 200 }
	offset := (page - 1) * limit
	searchTerm := "%" + q + "%"

	var total int
	DB.QueryRow(`
		SELECT COUNT(*) FROM books b
		LEFT JOIN categories c ON b.kategori_id = c.id
		WHERE b.judul ILIKE $1 OR b.kode ILIKE $1 OR b.keterangan ILIKE $1 OR c.nama ILIKE $1
	`, searchTerm).Scan(&total)
	totalPages := (total + limit - 1) / limit

	rows, err := DB.Query(`
		SELECT
			b.id, b.kode, b.judul, b.kategori_id, b.posisi_id, b.qty, b.keterangan, b.created_at, b.updated_at,
			c.nama as kategori_nama,
			p.kode as posisi_kode, p.rak as posisi_rak,
			CASE WHEN l.id IS NOT NULL THEN true ELSE false END as is_dipinjam,
			l.nama_peminjam as peminjam
		FROM books b
		LEFT JOIN categories c ON b.kategori_id = c.id
		LEFT JOIN posisi p     ON b.posisi_id    = p.id
		LEFT JOIN loans l      ON b.id = l.book_id AND l.tanggal_kembali IS NULL
		WHERE b.judul ILIKE $1 OR b.kode ILIKE $1 OR b.keterangan ILIKE $1 OR c.nama ILIKE $1
		ORDER BY b.judul ASC
		LIMIT $2 OFFSET $3
	`, searchTerm, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	books := []models.Book{}
	for rows.Next() {
		var b models.Book
		if err := rows.Scan(
			&b.ID, &b.Kode, &b.Judul, &b.KategoriID, &b.PosisiID, &b.Qty, &b.Keterangan, &b.CreatedAt, &b.UpdatedAt,
			&b.KategoriNama, &b.PosisiKode, &b.PosisiRak, &b.IsDipinjam, &b.Peminjam,
		); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		books = append(books, b)
	}

	return c.JSON(fiber.Map{
		"data":        books,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}


// GetBook - GET /api/books/:id
func GetBook(c *fiber.Ctx) error {
	id := c.Params("id")

	query := `
		SELECT 
			b.id, b.kode, b.judul, b.kategori_id, b.posisi_id, b.qty, b.keterangan, b.created_at, b.updated_at,
			c.nama as kategori_nama,
			p.kode as posisi_kode, p.rak as posisi_rak,
			CASE WHEN l.id IS NOT NULL THEN true ELSE false END as is_dipinjam,
			l.nama_peminjam as peminjam
		FROM books b
		LEFT JOIN categories c ON b.kategori_id = c.id
		LEFT JOIN posisi p ON b.posisi_id = p.id
		LEFT JOIN loans l ON b.id = l.book_id AND l.tanggal_kembali IS NULL
		WHERE b.id = $1
	`

	var b models.Book
	err := DB.QueryRow(query, id).Scan(
		&b.ID, &b.Kode, &b.Judul, &b.KategoriID, &b.PosisiID, &b.Qty, &b.Keterangan, &b.CreatedAt, &b.UpdatedAt,
		&b.KategoriNama, &b.PosisiKode, &b.PosisiRak, &b.IsDipinjam, &b.Peminjam,
	)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Book not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(b)
}

// CreateBook - POST /api/books
func CreateBook(c *fiber.Ctx) error {
	var input models.BookInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	if input.Judul == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Judul wajib diisi"})
	}

	if input.Qty == 0 {
		input.Qty = 1
	}

	query := `
		INSERT INTO books (kode, judul, kategori_id, posisi_id, qty, keterangan, created_by, created_by_nama)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	var id int
	err := DB.QueryRow(query, input.Kode, input.Judul, input.KategoriID, input.PosisiID, input.Qty, input.Keterangan, input.AdminID, input.AdminNama).Scan(&id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Get posisi and kategori names for logging
	var posisiKode, kategoriNama sql.NullString
	if input.PosisiID != nil {
		DB.QueryRow("SELECT kode FROM posisi WHERE id = $1", *input.PosisiID).Scan(&posisiKode)
	}
	if input.KategoriID != nil {
		DB.QueryRow("SELECT nama FROM categories WHERE id = $1", *input.KategoriID).Scan(&kategoriNama)
	}

	// Log activity
	adminNama := input.AdminNama
	if adminNama == "" {
		adminNama = "Unknown"
	}
	LogActivity(DB, input.AdminID, adminNama, models.ActionCreate, models.EntityBook, &id, &input.Judul, map[string]interface{}{
		"kode":     input.Kode,
		"qty":      input.Qty,
		"posisi":   posisiKode.String,
		"kategori": kategoriNama.String,
	})

	return c.Status(201).JSON(fiber.Map{"id": id, "message": "Book created"})
}

// UpdateBook - PUT /api/books/:id
func UpdateBook(c *fiber.Ctx) error {
	id := c.Params("id")

	var input models.BookInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	if input.Judul == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Judul wajib diisi"})
	}

	query := `
		UPDATE books 
		SET kode = $1, judul = $2, kategori_id = $3, posisi_id = $4, qty = $5, keterangan = $6, 
		    updated_at = NOW(), updated_by = $7, updated_by_nama = $8
		WHERE id = $9
	`

	result, err := DB.Exec(query, input.Kode, input.Judul, input.KategoriID, input.PosisiID, input.Qty, input.Keterangan, input.AdminID, input.AdminNama, id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Book not found"})
	}

	// Log activity
	adminNama := input.AdminNama
	if adminNama == "" {
		adminNama = "Unknown"
	}
	bookID, _ := strconv.Atoi(id)
	LogActivity(DB, input.AdminID, adminNama, models.ActionUpdate, models.EntityBook, &bookID, &input.Judul, map[string]interface{}{
		"kode":        input.Kode,
		"kategori_id": input.KategoriID,
		"posisi_id":   input.PosisiID,
		"qty":         input.Qty,
	})

	return c.JSON(fiber.Map{"message": "Book updated"})
}

// DeleteBook - DELETE /api/books/:id
// Requires superadmin approval (check via query param for now)
func DeleteBook(c *fiber.Ctx) error {
	id := c.Params("id")
	adminID := c.QueryInt("admin_id", 0)
	adminNama := c.Query("admin_nama", "Unknown")
	confirm := c.Query("confirm", "")
	alasan := c.Query("alasan", "")

	// Alasan wajib diisi
	if alasan == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Alasan penghapusan wajib diisi"})
	}

	// Get book info first for logging
	var bookJudul string
	err := DB.QueryRow("SELECT judul FROM books WHERE id = $1", id).Scan(&bookJudul)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "Book not found"})
	}

	bookID, _ := strconv.Atoi(id)
	logDetails := map[string]interface{}{"alasan": alasan}

	// Check if superadmin or has confirmation
	if confirm != "true" {
		// Log delete request
		LogActivity(DB, &adminID, adminNama, models.ActionDeleteRequest, models.EntityBook, &bookID, &bookJudul, logDetails)
		return c.Status(403).JSON(fiber.Map{
			"error":   "Penghapusan buku memerlukan konfirmasi superadmin",
			"message": "Permintaan hapus telah dicatat di log",
		})
	}

	result, err := DB.Exec("DELETE FROM books WHERE id = $1", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Book not found"})
	}

	// Log delete action
	LogActivity(DB, &adminID, adminNama, models.ActionDelete, models.EntityBook, &bookID, &bookJudul, logDetails)

	return c.JSON(fiber.Map{"message": "Book deleted"})
}

// InventoryCheck - POST /api/inventory/check
// Absen buku - tandai buku sudah dicek di posisinya
func InventoryCheck(c *fiber.Ctx) error {
	var input models.InventoryCheckInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	if len(input.BookIDs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Pilih minimal satu buku"})
	}

	if input.CheckedBy == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Nama petugas wajib diisi"})
	}

	// Update last_checked dan checked_by untuk semua buku yang dipilih
	// Jika ada new_posisi_id, update posisi juga
	var query string
	var args []interface{}

	if input.NewPosisiID != nil {
		query = `
			UPDATE books 
			SET last_checked = NOW(), checked_by = $1, posisi_id = $2, updated_at = NOW()
			WHERE id = ANY($3)
		`
		args = []interface{}{input.CheckedBy, *input.NewPosisiID, pq.Array(input.BookIDs)}
	} else {
		query = `
			UPDATE books 
			SET last_checked = NOW(), checked_by = $1, updated_at = NOW()
			WHERE id = ANY($2)
		`
		args = []interface{}{input.CheckedBy, pq.Array(input.BookIDs)}
	}

	result, err := DB.Exec(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	rowsAffected, _ := result.RowsAffected()

	// Log inventory check activity
	LogActivity(DB, nil, input.CheckedBy, models.ActionInventoryCheck, models.EntityBook, nil, nil, map[string]interface{}{
		"book_ids":      input.BookIDs,
		"new_posisi_id": input.NewPosisiID,
		"count":         rowsAffected,
	})

	return c.JSON(fiber.Map{
		"message": "Berhasil mengecek buku",
		"checked": rowsAffected,
	})
}

// GetBooksByPosisi - GET /api/inventory/posisi/:id
// Ambil semua buku di posisi tertentu untuk absen
func GetBooksByPosisi(c *fiber.Ctx) error {
	posisiID := c.Params("id")

	query := `
		SELECT 
			b.id, b.kode, b.judul, b.kategori_id, b.posisi_id, b.qty, b.keterangan, 
			b.created_at, b.updated_at, b.last_checked, b.checked_by,
			c.nama as kategori_nama,
			p.kode as posisi_kode, p.rak as posisi_rak,
			CASE WHEN l.id IS NOT NULL THEN true ELSE false END as is_dipinjam,
			l.nama_peminjam as peminjam
		FROM books b
		LEFT JOIN categories c ON b.kategori_id = c.id
		LEFT JOIN posisi p ON b.posisi_id = p.id
		LEFT JOIN loans l ON b.id = l.book_id AND l.tanggal_kembali IS NULL
		WHERE b.posisi_id = $1
		ORDER BY b.kode ASC, b.judul ASC
	`

	rows, err := DB.Query(query, posisiID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	books := []models.Book{}
	for rows.Next() {
		var b models.Book
		err := rows.Scan(
			&b.ID, &b.Kode, &b.Judul, &b.KategoriID, &b.PosisiID, &b.Qty, &b.Keterangan,
			&b.CreatedAt, &b.UpdatedAt, &b.LastChecked, &b.CheckedBy,
			&b.KategoriNama, &b.PosisiKode, &b.PosisiRak, &b.IsDipinjam, &b.Peminjam,
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		books = append(books, b)
	}

	return c.JSON(books)
}

// GetInventoryStats - GET /api/inventory/stats
// Statistik inventory: berapa buku sudah dicek, belum dicek
func GetInventoryStats(c *fiber.Ctx) error {
	var stats struct {
		Total        int `json:"total"`
		Checked      int `json:"checked"`
		Unchecked    int `json:"unchecked"`
		CheckedToday int `json:"checked_today"`
	}

	// Total buku
	DB.QueryRow("SELECT COUNT(*) FROM books").Scan(&stats.Total)

	// Sudah pernah dicek
	DB.QueryRow("SELECT COUNT(*) FROM books WHERE last_checked IS NOT NULL").Scan(&stats.Checked)

	// Belum pernah dicek
	stats.Unchecked = stats.Total - stats.Checked

	// Dicek hari ini
	DB.QueryRow("SELECT COUNT(*) FROM books WHERE last_checked::date = CURRENT_DATE").Scan(&stats.CheckedToday)

	return c.JSON(stats)
}
