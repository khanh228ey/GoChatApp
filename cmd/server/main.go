// Package main là điểm khởi động (entry point) của toàn bộ server.
// File này chỉ làm nhiệm vụ: load config → connect DB → chạy HTTP server.
package main

import (
	"log"

	"go_service/internal/app"
	"go_service/internal/config"
	"go_service/internal/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	mongo, err := config.ConnectMongo(cfg)
	if err != nil {
		log.Fatalf("failed to connect mongodb: %v", err)
	}
	defer func() {
		if err := mongo.Disconnect(); err != nil {
			log.Printf("failed to disconnect mongodb: %v", err)
		}
	}()
	log.Printf("connected to mongodb database: %s", cfg.MongoDatabase)

	application := app.New(cfg, mongo.Database)

	r := gin.Default()
	routes.Setup(r, application)

	log.Printf("server running on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
