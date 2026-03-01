package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// DashboardStats - GET /api/dashboard/stats
// Statistik untuk admin dashboard
func GetDashboardStats(c *fiber.Ctx) error {
	stats := fiber.Map{}

	// Total buku
	var totalBooks int
	DB.QueryRow("SELECT COUNT(*) FROM books").Scan(&totalBooks)
	stats["total_books"] = totalBooks

	// Total judul unik (berdasarkan qty)
	var totalQty int
	DB.QueryRow("SELECT COALESCE(SUM(qty), 0) FROM books").Scan(&totalQty)
	stats["total_qty"] = totalQty

	// Buku dipinjam
	var borrowed int
	DB.QueryRow("SELECT COUNT(*) FROM loans WHERE tanggal_kembali IS NULL").Scan(&borrowed)
	stats["borrowed"] = borrowed

	// Buku tersedia
	stats["available"] = totalBooks - borrowed

	// Total kategori
	var totalCategories int
	DB.QueryRow("SELECT COUNT(*) FROM categories").Scan(&totalCategories)
	stats["total_categories"] = totalCategories

	// Buku tanpa kode
	var withoutCode int
	DB.QueryRow("SELECT COUNT(*) FROM books WHERE kode IS NULL OR kode = ''").Scan(&withoutCode)
	stats["without_code"] = withoutCode

	// Buku sudah dicek inventory
	var checkedBooks int
	DB.QueryRow("SELECT COUNT(*) FROM books WHERE last_checked IS NOT NULL").Scan(&checkedBooks)
	stats["inventory_checked"] = checkedBooks

	// Buku dicek hari ini
	var checkedToday int
	DB.QueryRow("SELECT COUNT(*) FROM books WHERE last_checked::date = CURRENT_DATE").Scan(&checkedToday)
	stats["checked_today"] = checkedToday

	return c.JSON(stats)
}

// GetTopCategories - GET /api/dashboard/top-categories
func GetTopCategories(c *fiber.Ctx) error {
	rows, err := DB.Query(`
		SELECT c.nama, COUNT(b.id) as count 
		FROM categories c 
		LEFT JOIN books b ON c.id = b.kategori_id 
		GROUP BY c.id, c.nama 
		ORDER BY count DESC 
		LIMIT 10
	`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type CategoryCount struct {
		Nama  string `json:"nama"`
		Count int    `json:"count"`
	}

	categories := []CategoryCount{}
	for rows.Next() {
		var cat CategoryCount
		rows.Scan(&cat.Nama, &cat.Count)
		categories = append(categories, cat)
	}

	return c.JSON(categories)
}

// GetRecentLoans - GET /api/dashboard/recent-loans
func GetRecentLoans(c *fiber.Ctx) error {
	rows, err := DB.Query(`
		SELECT l.id, l.nama_peminjam, l.tanggal_pinjam, b.judul
		FROM loans l
		JOIN books b ON l.book_id = b.id
		WHERE l.tanggal_kembali IS NULL
		ORDER BY l.tanggal_pinjam DESC
		LIMIT 5
	`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type RecentLoan struct {
		ID            int    `json:"id"`
		NamaPeminjam  string `json:"nama_peminjam"`
		TanggalPinjam string `json:"tanggal_pinjam"`
		JudulBuku     string `json:"judul_buku"`
	}

	loans := []RecentLoan{}
	for rows.Next() {
		var loan RecentLoan
		rows.Scan(&loan.ID, &loan.NamaPeminjam, &loan.TanggalPinjam, &loan.JudulBuku)
		loans = append(loans, loan)
	}

	return c.JSON(loans)
}

// GetRecentActivity - GET /api/dashboard/recent-activity
func GetRecentActivity(c *fiber.Ctx) error {
	// Gabungan dari buku baru ditambah dan inventory check terbaru
	rows, err := DB.Query(`
		(SELECT 'book_added' as type, judul as title, created_at as time, NULL as actor FROM books ORDER BY created_at DESC LIMIT 5)
		UNION ALL
		(SELECT 'inventory_check' as type, judul as title, last_checked as time, checked_by as actor FROM books WHERE last_checked IS NOT NULL ORDER BY last_checked DESC LIMIT 5)
		ORDER BY time DESC
		LIMIT 10
	`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type Activity struct {
		Type  string  `json:"type"`
		Title string  `json:"title"`
		Time  string  `json:"time"`
		Actor *string `json:"actor"`
	}

	activities := []Activity{}
	for rows.Next() {
		var act Activity
		rows.Scan(&act.Type, &act.Title, &act.Time, &act.Actor)
		activities = append(activities, act)
	}

	return c.JSON(activities)
}
