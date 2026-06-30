// Package main là điểm khởi động (entry point) của toàn bộ server.
// File này chỉ làm nhiệm vụ: load config → connect DB → khởi tạo socket → chạy HTTP server.
package main

import (
	"log"

	"go_service/internal/config"
	"go_service/internal/handler"
	"go_service/internal/repository"
	"go_service/internal/routes"
	"go_service/internal/service"
	"go_service/internal/socket"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Đọc biến môi trường từ file .env
	cfg := config.Load()

	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	// 2. Kết nối MongoDB và kiểm tra ping
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

	// 3. Khởi tạo repository → service → handler (auth)
	userRepo := repository.NewUserRepository(mongo.Database)
	refreshTokenRepo := repository.NewRefreshTokenRepository(mongo.Database)
	authService := service.NewAuthService(userRepo, refreshTokenRepo, cfg)
	authHandler := handler.NewAuthHandler(authService)

	// 3b. Friendship layer
	friendshipRepo := repository.NewFriendshipRepository(mongo.Database)
	friendshipService := service.NewFriendshipService(userRepo, friendshipRepo)
	friendshipHandler := handler.NewFriendshipHandler(friendshipService)

	// 4. Khởi tạo WebSocket Hub và chạy trong goroutine riêng
	hub := socket.NewHub()
	go hub.Run()
	socketHandler := socket.NewHandler(hub)

	// 5. Khởi tạo Gin router và đăng ký tất cả routes
	r := gin.Default()
	routes.Setup(r, cfg, socketHandler, authHandler, friendshipHandler, authService)

	// 6. Chạy HTTP server trên port đã cấu hình
	log.Printf("server running on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
