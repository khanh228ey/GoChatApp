// Package main là điểm khởi động (entry point) của toàn bộ server.
// File này chỉ làm nhiệm vụ: load config → connect DB → khởi tạo socket → chạy HTTP server.
package main

import (
	"log"

	"go_service/internal/config"
	"go_service/internal/routes"
	"go_service/internal/socket"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Đọc biến môi trường từ file .env (PORT, MONGO_URI, MONGO_DATABASE)
	cfg := config.Load()

	// 2. Kết nối MongoDB và kiểm tra ping
	mongo, err := config.ConnectMongo(cfg)
	if err != nil {
		log.Fatalf("failed to connect mongodb: %v", err)
	}
	// Đóng kết nối MongoDB khi server tắt
	defer func() {
		if err := mongo.Disconnect(); err != nil {
			log.Printf("failed to disconnect mongodb: %v", err)
		}
	}()
	log.Printf("connected to mongodb database: %s", cfg.MongoDatabase)

	// 3. Khởi tạo WebSocket Hub và chạy trong goroutine riêng
	//    Hub quản lý danh sách client đang kết nối và broadcast message
	hub := socket.NewHub()
	go hub.Run()

	// 4. Tạo handler xử lý kết nối WebSocket từ client
	socketHandler := socket.NewHandler(hub)

	// 5. Khởi tạo Gin router và đăng ký tất cả routes
	r := gin.Default()
	routes.Setup(r, socketHandler)

	// 6. Chạy HTTP server trên port đã cấu hình
	log.Printf("server running on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
