// Package config chứa cấu hình ứng dụng, đọc từ biến môi trường (.env).
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config lưu toàn bộ cấu hình cần thiết cho server.
type Config struct {
	Port          string // Port HTTP server lắng nghe (mặc định: 8080)
	MongoURI      string // URI kết nối MongoDB (vd: mongodb://localhost:27017)
	MongoDatabase string // Tên database sử dụng trong MongoDB
}

// Load đọc file .env và trả về struct Config.
// Nếu không tìm thấy .env, vẫn chạy được nhưng dùng biến môi trường hệ thống.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		Port:          port,
		MongoURI:      os.Getenv("MONGO_URI"),
		MongoDatabase: os.Getenv("MONGO_DATABASE"),
	}
}
