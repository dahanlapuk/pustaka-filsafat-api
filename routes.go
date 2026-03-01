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

	api.Get("/books", handlers.GetBooks)
	api.Get("/books/search", handlers.SearchBooks)
	api.Get("/books/:id", handlers.GetBook)
	api.Get("/categories", handlers.GetCategories)
	api.Get("/posisi", handlers.GetPosisi)

	api.Post("/books", handlers.CreateBook)
	api.Put("/books/:id", handlers.UpdateBook)
	api.Delete("/books/:id", handlers.DeleteBook)
	api.Get("/books/no-code", handlers.GetBooksWithoutCode(DB))
	api.Post("/books/:id/generate-code", handlers.GenerateBookCode(DB))

	api.Get("/loans", handlers.GetLoans)
	api.Post("/loans", handlers.CreateLoan)
	api.Put("/loans/:id/return", handlers.ReturnLoan)

	api.Get("/inventory/stats", handlers.GetInventoryStats)
	api.Get("/inventory/posisi/:id", handlers.GetBooksByPosisi)
	api.Post("/inventory/check", handlers.InventoryCheck)

	api.Get("/dashboard/stats", handlers.GetDashboardStats)
	api.Get("/dashboard/top-categories", handlers.GetTopCategories)
	api.Get("/dashboard/recent-loans", handlers.GetRecentLoans)
	api.Get("/dashboard/recent-activity", handlers.GetRecentActivity)

	api.Get("/admins", handlers.GetAdmins(DB))
	api.Get("/admins/current", handlers.GetCurrentAdmin(DB))
	api.Post("/admins/login", handlers.LoginAdmin(DB))
	api.Post("/admins/logout", handlers.LogoutAdmin(DB))
	api.Put("/admins/:id/profile", handlers.UpdateProfile(DB))
	api.Put("/admins/:id/password", handlers.ChangePassword(DB))

	api.Get("/logs", handlers.GetActivityLogs(DB))
	api.Get("/logs/stats", handlers.GetLogStats(DB))
}
