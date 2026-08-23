// Package config chứa cấu hình ứng dụng, đọc từ biến môi trường (.env).
package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config lưu toàn bộ cấu hình cần thiết cho server.
type Config struct {
	Port                     string // Port HTTP server lắng nghe (mặc định: 8080)
	MongoURI                 string // URI kết nối MongoDB
	MongoDatabase            string // Tên database sử dụng trong MongoDB
	JWTSecret                string // Secret key để ký JWT access token
	AccessTokenExpireMinutes int    // Thời hạn access token (phút, mặc định: 15)
	RefreshTokenExpireDays   int    // Thời hạn refresh token (ngày, mặc định: 7)
	FrontendOrigin           string // Origin cho phép CORS (vd: http://localhost:5173)

	KafkaBrokers            []string // Danh sách broker Kafka (mặc định: localhost:9094)
	KafkaMessagesTopic      string   // Topic chứa event tin nhắn mới (mặc định: chat.messages)
	KafkaMessagesPartitions int      // Số partition khi tạo topic (mặc định: 3)
	KafkaReplicationFactor  int      // Replication factor khi tạo topic (mặc định: 1, phù hợp 1 broker)
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

	expireMinutes := 15
	if v := os.Getenv("ACCESS_TOKEN_EXPIRE_MINUTES"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			expireMinutes = parsed
		}
	}

	expireDays := 7
	if v := os.Getenv("REFRESH_TOKEN_EXPIRE_DAYS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			expireDays = parsed
		}
	}

	frontendOrigin := os.Getenv("FRONTEND_ORIGIN")
	if frontendOrigin == "" {
		frontendOrigin = "http://localhost:5173"
	}

	kafkaBrokers := []string{"localhost:9094"}
	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		kafkaBrokers = strings.Split(v, ",")
	}

	kafkaMessagesTopic := os.Getenv("KAFKA_MESSAGES_TOPIC")
	if kafkaMessagesTopic == "" {
		kafkaMessagesTopic = "chat.messages"
	}

	kafkaPartitions := 3
	if v := os.Getenv("KAFKA_MESSAGES_PARTITIONS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			kafkaPartitions = parsed
		}
	}

	kafkaReplicationFactor := 1
	if v := os.Getenv("KAFKA_REPLICATION_FACTOR"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			kafkaReplicationFactor = parsed
		}
	}

	return &Config{
		Port:                     port,
		MongoURI:                 os.Getenv("MONGO_URI"),
		MongoDatabase:            os.Getenv("MONGO_DATABASE"),
		JWTSecret:                os.Getenv("JWT_SECRET"),
		AccessTokenExpireMinutes: expireMinutes,
		RefreshTokenExpireDays:   expireDays,
		FrontendOrigin:           frontendOrigin,

		KafkaBrokers:            kafkaBrokers,
		KafkaMessagesTopic:      kafkaMessagesTopic,
		KafkaMessagesPartitions: kafkaPartitions,
		KafkaReplicationFactor:  kafkaReplicationFactor,
	}
}
