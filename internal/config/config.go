// Package config chứa cấu hình ứng dụng, đọc từ biến môi trường (.env).
package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config lưu toàn bộ cấu hình cần thiết cho server.
type Config struct {
	Port           string // Port HTTP server lắng nghe (mặc định: 8080)
	MongoURI       string // URI kết nối MongoDB (vd: mongodb://localhost:27017)
	MongoDatabase  string // Tên database sử dụng trong MongoDB
	JWTSecret      string // Secret key để ký JWT token
	JWTExpireHours int    // Thời gian hết hạn token (giờ)
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

	jwtExpireHours := 24
	if v := os.Getenv("JWT_EXPIRE_HOURS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			jwtExpireHours = parsed
		}
	}

	return &Config{
		Port:           port,
		MongoURI:       os.Getenv("MONGO_URI"),
		MongoDatabase:  os.Getenv("MONGO_DATABASE"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		JWTExpireHours: jwtExpireHours,
	}
}
