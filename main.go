package main

import (
	"log"

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

	app := fiber.New(fiber.Config{
		AppName: "Pustaka Filsafat API",
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Admin-ID, X-Session-Start",
		AllowMethods: "GET, POST, PUT, DELETE",
	}))

	app.Use(middleware.AdminAuth(DB))
	SetupRoutes(app)
	log.Println("🚀 Server running on http://0.0.0.0:3000")
	log.Fatal(app.Listen("0.0.0.0:3000"))
}
