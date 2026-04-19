package main

import (
	"pustaka-filsafat/handlers"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api")

	// ── Books ────────────────────────────────────────────────
	api.Get("/books", handlers.GetBooks)
	api.Get("/books/search", handlers.SearchBooks)
	api.Get("/books/no-code", handlers.GetBooksWithoutCode(DB))
	api.Get("/books/:id", handlers.GetBook)
	api.Post("/books", handlers.CreateBook)
	api.Put("/books/batch-posisi", handlers.BatchUpdatePosisi)
	api.Put("/books/:id", handlers.UpdateBook)
	api.Put("/books/:id/posisi", handlers.UpdatePosisiBuku)
	api.Delete("/books/:id", handlers.DeleteBook)
	api.Post("/books/:id/generate-code", handlers.GenerateBookCode(DB))

	// ── Posisi ───────────────────────────────────────────────
	api.Get("/posisi", handlers.GetPosisi)
	api.Get("/posisi/struktur", handlers.GetPosisiStruktur)

	// ── Categories ───────────────────────────────────────────
	api.Get("/categories", handlers.GetCategories)
	api.Post("/categories", handlers.CreateCategory)
	api.Put("/categories/:id", handlers.UpdateCategory)
	api.Delete("/categories/:id", handlers.DeleteCategory)

	// ── Category Requests ────────────────────────────────────
	api.Get("/category-requests", handlers.GetCategoryRequests)
	api.Post("/category-requests", handlers.CreateCategoryRequest)
	api.Put("/category-requests/:id/approve", handlers.ApproveCategoryRequest)
	api.Put("/category-requests/:id/reject", handlers.RejectCategoryRequest)

	// ── Loans ────────────────────────────────────────────────
	api.Get("/loans", handlers.GetLoans)
	api.Post("/loans", handlers.CreateLoan)
	api.Put("/loans/:id/return", handlers.ReturnLoan)

	// ── Loan Requests (POST publik, GET/PUT protected) ───────
	api.Get("/loan-requests", handlers.GetLoanRequests)
	api.Post("/loan-requests", handlers.CreateLoanRequest)
	api.Put("/loan-requests/:id/approve", handlers.ApproveLoanRequest)
	api.Put("/loan-requests/:id/reject", handlers.RejectLoanRequest)

	// ── Delete Requests ──────────────────────────────────────
	api.Get("/delete-requests", handlers.GetDeleteRequests)
	api.Post("/delete-requests", handlers.CreateDeleteRequest)
	api.Put("/delete-requests/:id/approve", handlers.ApproveDeleteRequest)
	api.Put("/delete-requests/:id/reject", handlers.RejectDeleteRequest)

	// ── Inventory ────────────────────────────────────────────
	api.Get("/inventory/stats", handlers.GetInventoryStats)
	api.Get("/inventory/posisi/:id", handlers.GetBooksByPosisi)
	api.Post("/inventory/check", handlers.InventoryCheck)

	// ── Dashboard ────────────────────────────────────────────
	api.Get("/dashboard/stats", handlers.GetDashboardStats)
	api.Get("/dashboard/top-categories", handlers.GetTopCategories)
	api.Get("/dashboard/recent-loans", handlers.GetRecentLoans)
	api.Get("/dashboard/recent-activity", handlers.GetRecentActivity)

	// ── Admin (users table) ──────────────────────────────────
	api.Get("/admins", handlers.GetAdmins(DB))
	api.Get("/admins/current", handlers.GetCurrentAdmin(DB))
	api.Post("/admins/login", handlers.LoginAdmin(DB))
	api.Post("/admins/logout", handlers.LogoutAdmin(DB))
	api.Post("/admins", handlers.CreateAdmin(DB))
	api.Put("/admins/:id/admin", handlers.UpdateAdminBySuper(DB))
	api.Delete("/admins/:id", handlers.DeleteAdmin(DB))
	api.Put("/admins/:id/profile", handlers.UpdateProfile(DB))
	api.Put("/admins/:id/password", handlers.ChangePassword(DB))

	// ── Activity Logs ────────────────────────────────────────
	api.Get("/logs", handlers.GetActivityLogs(DB))
	api.Get("/logs/stats", handlers.GetLogStats(DB))
}
