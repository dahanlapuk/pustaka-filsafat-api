package handlers

import (
	"github.com/gofiber/fiber/v2"
)

func GetMemberArchive(c *fiber.Ctx) error {
	q := c.Query("q")
	query := `
		SELECT nama_peminjam, COUNT(*) AS total_borrow, MAX(tanggal_pinjam) AS last_borrow
		FROM loans
		WHERE 1=1
	`
	args := []interface{}{}
	if q != "" {
		query += " AND nama_peminjam ILIKE $1"
		args = append(args, "%"+q+"%")
	}
	query += " GROUP BY nama_peminjam ORDER BY total_borrow DESC, last_borrow DESC LIMIT 200"

	rows, err := DB.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	members := []fiber.Map{}
	for rows.Next() {
		var name string
		var total int
		var lastBorrow string
		if err := rows.Scan(&name, &total, &lastBorrow); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		members = append(members, fiber.Map{
			"name":         name,
			"total_borrow": total,
			"last_borrow":  lastBorrow,
		})
	}

	return c.JSON(fiber.Map{"members": members})
}

func GetMemberProfile(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Nama anggota wajib diisi"})
	}

	var summary struct {
		Name         string  `json:"name"`
		TotalBorrow  int     `json:"total_borrow"`
		ActiveBorrow int     `json:"active_borrow"`
		LastBorrow   *string `json:"last_borrow,omitempty"`
	}
	if err := DB.QueryRow(`
		SELECT nama_peminjam, COUNT(*) AS total_borrow,
		       COUNT(*) FILTER (WHERE tanggal_kembali IS NULL) AS active_borrow,
		       MAX(tanggal_pinjam)::text AS last_borrow
		FROM loans
		WHERE nama_peminjam = $1
		GROUP BY nama_peminjam
	`, name).Scan(&summary.Name, &summary.TotalBorrow, &summary.ActiveBorrow, &summary.LastBorrow); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Profil anggota tidak ditemukan"})
	}

	rows, err := DB.Query(`
		SELECT l.id, l.book_id, b.judul, l.tanggal_pinjam::text, l.tanggal_kembali::text, l.catatan
		FROM loans l
		JOIN books b ON b.id = l.book_id
		WHERE l.nama_peminjam = $1
		ORDER BY l.tanggal_pinjam DESC
		LIMIT 200
	`, name)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	history := []fiber.Map{}
	for rows.Next() {
		var id, bookID int
		var title, pinjam string
		var kembali, note *string
		if err := rows.Scan(&id, &bookID, &title, &pinjam, &kembali, &note); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		history = append(history, fiber.Map{
			"loan_id":         id,
			"book_id":         bookID,
			"book_title":      title,
			"tanggal_pinjam":  pinjam,
			"tanggal_kembali": kembali,
			"catatan":         note,
		})
	}

	return c.JSON(fiber.Map{
		"profile": summary,
		"history": history,
	})
}
