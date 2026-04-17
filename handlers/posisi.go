package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// GetPosisi - GET /api/posisi
func GetPosisi(c *fiber.Ctx) error {
	rows, err := DB.Query(`
		SELECT id, kode, rak, rak_no, baris, kolom_no, letak, deskripsi
		FROM posisi
		ORDER BY rak_no ASC, baris ASC, kolom_no ASC
	`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type Posisi struct {
		ID          int     `json:"id"`
		Kode        string  `json:"kode"`
		Rak         string  `json:"rak"`
		RakNo       int     `json:"rak_no"`
		Baris       string  `json:"baris"`
		KolomNo     *int    `json:"kolom_no"`
		Letak       *string `json:"letak"`
		Deskripsi   *string `json:"deskripsi"`
	}

	list := []Posisi{}
	for rows.Next() {
		var p Posisi
		if err := rows.Scan(&p.ID, &p.Kode, &p.Rak, &p.RakNo, &p.Baris, &p.KolomNo, &p.Letak, &p.Deskripsi); err != nil {
			continue
		}
		list = append(list, p)
	}
	return c.JSON(list)
}

// GetPosisiStruktur - GET /api/posisi/struktur
// Mengembalikan posisi yang dikelompokkan per rak
func GetPosisiStruktur(c *fiber.Ctx) error {
	rows, err := DB.Query(`
		SELECT id, kode, rak, rak_no, baris, kolom_no, letak
		FROM posisi
		ORDER BY rak_no ASC, baris ASC, kolom_no ASC
	`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type PosisiItem struct {
		ID      int     `json:"id"`
		Kode    string  `json:"kode"`
		Rak     string  `json:"rak"`
		RakNo   int     `json:"rak_no"`
		Baris   string  `json:"baris"`
		KolomNo *int    `json:"kolom_no"`
		Letak   *string `json:"letak"`
	}

	type RakGroup struct {
		RakNo  int          `json:"rak_no"`
		Nama   string       `json:"nama"`
		Items  []PosisiItem `json:"items"`
	}

	allItems := []PosisiItem{}
	for rows.Next() {
		var p PosisiItem
		if err := rows.Scan(&p.ID, &p.Kode, &p.Rak, &p.RakNo, &p.Baris, &p.KolomNo, &p.Letak); err != nil {
			continue
		}
		allItems = append(allItems, p)
	}

	// Group by rak_no
	rakMap := map[int]*RakGroup{}
	rakOrder := []int{}
	for _, item := range allItems {
		if _, ok := rakMap[item.RakNo]; !ok {
			rakMap[item.RakNo] = &RakGroup{RakNo: item.RakNo, Nama: item.Rak, Items: []PosisiItem{}}
			rakOrder = append(rakOrder, item.RakNo)
		}
		rakMap[item.RakNo].Items = append(rakMap[item.RakNo].Items, item)
	}

	result := []RakGroup{}
	for _, no := range rakOrder {
		result = append(result, *rakMap[no])
	}
	return c.JSON(result)
}

// UpdatePosisiBuku - PUT /api/books/:id/posisi
// Update posisi satu buku
func UpdatePosisiBuku(c *fiber.Ctx) error {
	bookID := c.Params("id")
	var input struct {
		PosisiID  *int   `json:"posisi_id"`
		AdminID   int    `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}

	_, err := DB.Exec(`
		UPDATE books SET posisi_id = $1, updated_at = NOW(), updated_by = $2, updated_by_nama = $3
		WHERE id = $4
	`, input.PosisiID, input.AdminID, input.AdminNama, bookID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Posisi buku berhasil diupdate", "book_id": bookID, "posisi_id": input.PosisiID})
}

// UpdatePosisiBukuBatch - PUT /api/books/posisi-batch
// Update posisi banyak buku sekaligus
func UpdatePosisiBukuBatch(c *fiber.Ctx) error {
	var input struct {
		Updates []struct {
			BookID   int  `json:"book_id"`
			PosisiID *int `json:"posisi_id"`
		} `json:"updates"`
		AdminID   int    `json:"admin_id"`
		AdminNama string `json:"admin_nama"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
	}
	if len(input.Updates) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Tidak ada data update"})
	}

	updated := 0
	for _, u := range input.Updates {
		res, err := DB.Exec(`
			UPDATE books SET posisi_id = $1, updated_at = NOW(), updated_by = $2, updated_by_nama = $3
			WHERE id = $4
		`, u.PosisiID, input.AdminID, input.AdminNama, u.BookID)
		if err == nil {
			n, _ := res.RowsAffected()
			updated += int(n)
		}
	}

	return c.JSON(fiber.Map{"message": "Selesai update posisi", "updated": updated})
}
