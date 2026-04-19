package main

import (
	"log"
	"os"

	"pustaka-filsafat/handlers"
	"pustaka-filsafat/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	ConnectDB()
	handlers.SetDB(DB)
	if err := handlers.EnsureAuthSessionSchema(DB); err != nil {
		log.Fatal(err)
	}
	if err := handlers.EnsureActivityLogSchema(DB); err != nil {
		log.Fatal(err)
	}
	if err := handlers.EnsureBookTagSchema(DB); err != nil {
		log.Fatal(err)
	}
	if err := handlers.EnsureBookStockSchema(DB); err != nil {
		log.Fatal(err)
	}
	if err := handlers.EnsureStocktakeSchema(DB); err != nil {
		log.Fatal(err)
	}

	app := fiber.New(fiber.Config{
		AppName: "Pustaka Filsafat API",
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		// Keep legacy headers during transition so old frontend builds don't fail preflight.
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Session-Token, X-Admin-ID, X-Session-Start",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	app.Use(middleware.AdminAuth(DB))
	SetupRoutes(app)

	// Railway/Render assign PORT dinamis; lokal default 3000
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("🚀 Server running on port %s", port)
	log.Fatal(app.Listen("0.0.0.0:" + port))
}
